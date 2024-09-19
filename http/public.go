package http

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/v2/files"
	"github.com/versioneer-tech/package-r/v2/objects"
	"github.com/versioneer-tech/package-r/v2/share"
)

type LinkData struct {
	FileInfo files.FileInfo
	Source   share.Source
}

var withHashFile = func(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		id, ifPath := ifPathWithName(r)
		link, err := d.store.Share.GetByHash(id)
		if err != nil {
			return errToStatus(err), err
		}

		status, err := authenticateShareRequest(r, link)
		if status != 0 || err != nil {
			return status, err
		}

		d.user, err = d.store.Users.Get(link.UserID)
		if err != nil {
			return errToStatus(err), err
		}

		bucket, prefix, session := link.Source.Connect(*d.store.K8sCache)
		if session != nil {
			d.user.Fs = afero.NewBasePathFs(objects.NewObjectFs(bucket, session), "/"+prefix)
		}

		fileInfo, err := files.NewFileInfo(&files.FileOptions{
			Fs:      d.user.Fs,
			Path:    link.Path,
			Modify:  d.user.Perm.Modify,
			Expand:  false,
			Checker: d,
			Token:   link.Token,
		})
		if err != nil {
			return errToStatus(err), err
		}

		// share base path
		basePath := link.Path

		// file relative path
		filePath := ""

		if fileInfo.IsDir {
			basePath = filepath.Dir(basePath)
			filePath = ifPath
		}

		// set fs root to the shared file/folder
		d.user.Fs = afero.NewBasePathFs(d.user.Fs, basePath)

		token := link.Token

		fileInfo, err = files.NewFileInfo(&files.FileOptions{
			Fs:      d.user.Fs,
			Path:    filePath,
			Modify:  d.user.Perm.Modify,
			Expand:  true,
			Checker: d,
			Token:   token,
		})
		if err != nil {
			return errToStatus(err), err
		}

		linkData := LinkData{
			FileInfo: *fileInfo,
			Source:   link.Source,
		}

		d.raw = linkData
		return fn(w, r, d)
	}
}

// ref to https://github.com/versioneer-tech/package-r/pull/727
// `/api/public/dl/MEEuZK-v/file-name.txt` for old browsers to save file with correct name
func ifPathWithName(r *http.Request) (id, filePath string) {
	pathElements := strings.Split(r.URL.Path, "/")
	// prevent maliciously constructed parameters like `/api/public/dl/XZzCDnK2_not_exists_hash_name`
	// len(pathElements) will be 1, and golang will panic `runtime error: index out of range`

	switch len(pathElements) {
	case 1:
		return r.URL.Path, "/"
	default:
		return pathElements[0], path.Join("/", path.Join(pathElements[1:]...))
	}
}

var publicShareHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	linkData := d.raw.(LinkData)
	fileInfo := linkData.FileInfo

	if fileInfo.IsDir {
		fileInfo.Listing.Sorting = files.Sorting{By: "name", Asc: false}
		fileInfo.Listing.ApplySort()
		return renderJSON(w, r, fileInfo)
	}

	presignedURLs, _, err := share.Presign(&linkData.Source, *d.store.K8sCache, fileInfo.RealPath())
	if err == nil {
		fileInfo.PresignedURL = presignedURLs[0]
	}
	return renderJSON(w, r, fileInfo)
})

var publicDlHandler = withHashFile(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	linkData := d.raw.(LinkData)
	fileInfo := linkData.FileInfo

	var paths []string
	if fileInfo.IsDir {
		var wg sync.WaitGroup
		pathChan := make(chan string)
		for _, subFileInfo := range fileInfo.Listing.Items {
			wg.Add(1)
			if subFileInfo.IsDir {
				go func() {
					defer (&wg).Done()

					subFile, err := fileInfo.Fs.Open(subFileInfo.Path)
					if err != nil {
						log.Printf("error opening %s preparing presign: %s", subFileInfo.Path, err)
						return
					}
					defer subFile.Close()

					maxObjects, err := strconv.Atoi(os.Getenv("MAX_OBJECTS"))
					if err != nil || maxObjects < 0 {
						maxObjects = 5000
					}
					batch := 1000
					steps := (maxObjects / batch) + 1
					for step := range steps {
						fileInfos, err2 := subFile.Readdir(-batch)
						if err2 != nil {
							if errors.Is(err2, io.EOF) {
								break
							}
							log.Printf("error reading dir at %s preparing presign: %s", subFileInfo.Path, err2)
							return
						}
						if len(fileInfos) == 0 {
							break
						}
						log.Printf("Prepare presign in %s for %v items (%d/%d)", subFile.Name(), len(fileInfos), step, steps)
						for _, fileInfo := range fileInfos {
							objectInfo, ok := fileInfo.(objects.ObjectInfo)
							if ok {
								chan<- string(pathChan) <- objectInfo.Path()
							}
						}
					}
				}()
			} else {
				go func() {
					defer wg.Done()
					pathChan <- subFileInfo.RealPath()
				}()
			}
		}

		go func() {
			wg.Wait()
			close(pathChan)
		}()
		for path := range pathChan {
			paths = append(paths, path)
		}

	} else {
		paths = append(paths, fileInfo.RealPath())
	}

	log.Printf("Start presign (total %v)", len(paths))
	presignedURLs, status, err := share.Presign(&linkData.Source, *d.store.K8sCache, paths...)

	if err != nil {
		return status, err
	}

	if len(presignedURLs) == 0 {
		return http.StatusNotFound, nil
	}

	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		err = encoder.Encode(presignedURLs)
		if err != nil {
			return http.StatusInternalServerError, err
		}
	} else if len(presignedURLs) == 1 {
		http.Redirect(w, r, presignedURLs[0], http.StatusFound)
	} else {
		reader := strings.NewReader(strings.Join(presignedURLs, "\n"))
		filename := url.PathEscape(strings.ReplaceAll(d.settings.Branding.Name+"/"+fileInfo.Path+".txt", "/", "__"))
		log.Printf("return presign file '%s'", filename)
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+filename)
		w.Header().Add("Content-Security-Policy", `script-src 'none';`)
		w.Header().Set("Cache-Control", "private")
		http.ServeContent(w, r, filename, time.Now(), reader)
	}
	return 0, nil
})

func authenticateShareRequest(r *http.Request, l *share.Link) (int, error) {
	if l.PasswordHash == "" {
		return 0, nil
	}

	if r.URL.Query().Get("token") == l.Token {
		return 0, nil
	}

	password := r.Header.Get("X-SHARE-PASSWORD")
	password, err := url.QueryUnescape(password)
	if err != nil {
		return 0, err
	}
	if password == "" {
		return http.StatusUnauthorized, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(l.PasswordHash), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return http.StatusUnauthorized, nil
		}
		return 0, err
	}

	return 0, nil
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}
