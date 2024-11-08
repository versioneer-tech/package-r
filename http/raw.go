package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	gopath "path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mholt/archiver/v3"

	"github.com/versioneer-tech/package-r/files"
	"github.com/versioneer-tech/package-r/fileutils"
	"github.com/versioneer-tech/package-r/users"
)

func slashClean(name string) string {
	if name == "" || name[0] != '/' {
		name = "/" + name
	}
	return gopath.Clean(name)
}

func parseQueryFiles(r *http.Request, f *files.FileInfo, _ *users.User) ([]string, error) {
	var fileSlice []string
	names := strings.Split(r.URL.Query().Get("files"), ",")

	if len(names) == 0 {
		fileSlice = append(fileSlice, f.Path)
	} else {
		for _, name := range names {
			name, err := url.QueryUnescape(strings.Replace(name, "+", "%2B", -1)) //nolint:govet
			if err != nil {
				return nil, err
			}

			name = slashClean(name)
			fileSlice = append(fileSlice, filepath.Join(f.Path, name))
		}
	}

	return fileSlice, nil
}

func parseQueryAlgorithm(r *http.Request) (string, archiver.Writer, error) {
	switch r.URL.Query().Get("algo") {
	case "zip", "true", "":
		return ".zip", archiver.NewZip(), nil
	case "tar":
		return ".tar", archiver.NewTar(), nil
	case "targz":
		return ".tar.gz", archiver.NewTarGz(), nil
	case "tarbz2":
		return ".tar.bz2", archiver.NewTarBz2(), nil
	case "tarxz":
		return ".tar.xz", archiver.NewTarXz(), nil
	case "tarlz4":
		return ".tar.lz4", archiver.NewTarLz4(), nil
	case "tarsz":
		return ".tar.sz", archiver.NewTarSz(), nil
	default:
		return "", nil, errors.New("format not implemented")
	}
}

func setContentDisposition(w http.ResponseWriter, r *http.Request, file *files.FileInfo) {
	if r.URL.Query().Get("inline") == "true" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		// As per RFC6266 section 4.3
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(file.Name))
	}
}

var rawHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Download {
		return http.StatusAccepted, nil
	}

	file, err := files.NewFileInfo(&files.FileOptions{
		Fs:         d.user.Fs,
		Path:       r.URL.Path,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: d.server.TypeDetectionByHeader,
		Checker:    d,
	})
	if err != nil {
		return errToStatus(err), err
	}

	if files.IsNamedPipe(file.Mode) {
		setContentDisposition(w, r, file)
		return 0, nil
	}

	if !file.IsDir {
		return rawFileHandler(w, r, file)
	}

	return rawDirHandler(w, r, d, file)
})

func addFile(ar archiver.Writer, d *data, fpath, commonPath, downloadURLBase string) error {
	if !d.Check(fpath) {
		return nil
	}

	info, err := d.user.Fs.Stat(fpath)
	if err != nil {
		return err
	}

	if !info.IsDir() && !info.Mode().IsRegular() {
		return nil
	}

	file, err := d.user.Fs.Open(fpath)
	if err != nil {
		return err
	}
	defer file.Close()

	if fpath != commonPath {
		filename := strings.TrimPrefix(fpath, commonPath)
		filename = strings.TrimPrefix(filename, string(filepath.Separator))
		if pointerInfo, ok := info.(*files.PointerInfo); ok {
			filename += ".pointer"
			file = files.NewPointer(pointerInfo, downloadURLBase+fpath)
		}
		err = ar.Write(archiver.File{
			FileInfo: archiver.FileInfo{
				FileInfo:   info,
				CustomName: filename,
			},
			ReadCloser: file,
		})
		if err != nil {
			return err
		}
	}

	if info.IsDir() {
		names, err := file.Readdirnames(0)
		if err != nil {
			return err
		}

		for _, name := range names {
			fPath := filepath.Join(fpath, name)
			err = addFile(ar, d, fPath, commonPath, downloadURLBase)
			if err != nil {
				log.Printf("Failed to archive %s: %v", fPath, err)
			}
		}
	}

	return nil
}

func rawDirHandler(w http.ResponseWriter, r *http.Request, d *data, file *files.FileInfo) (int, error) {
	filenames, err := parseQueryFiles(r, file, d.user)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	extension, ar, err := parseQueryAlgorithm(r)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	err = ar.Create(w)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer ar.Close()

	commonDir := ""
	if !file.IsDir {
		commonDir = filepath.Dir(file.Path)
	} else {
		commonDir = fileutils.CommonPrefix(filepath.Separator, filenames...)
	}

	name := filepath.Base(commonDir)
	if name == "." || name == "" || name == string(filepath.Separator) {
		name = file.Name
	}
	// Prefix used to distinguish a filelist generated
	// archive from the full directory archive
	if len(filenames) > 1 {
		name = "_" + name
	}
	name += extension
	w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(name))

	downloadURLBase, err := url.Parse(GetRequestURI(r))
	if err == nil {
		escaped, err2 := url.PathUnescape(downloadURLBase.Path)
		if err2 == nil {
			// Regular expression to match `/public/dl/{hash}/`
			re := regexp.MustCompile(`(/public/dl/[^/]+)(/.*)?`)
			downloadURLBase.Path = re.ReplaceAllString(escaped, `$1`)
			downloadURLBase.RawQuery = ""
		}
	}

	for _, fname := range filenames {
		err = addFile(ar, d, fname, commonDir, downloadURLBase.String())
		if err != nil {
			log.Printf("Failed to archive %s: %v", fname, err)
		}
	}

	return 0, nil
}

func rawFileHandler(w http.ResponseWriter, r *http.Request, file *files.FileInfo) (int, error) { //nolint:gocyclo
	if file.Type == "pointer" {
		info, err := file.Fs.Stat(file.Path)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if pinfo, ok := info.(*files.PointerInfo); ok {
			var b strings.Builder
			relpath := pinfo.Filepath
			if pinfo.Linkpath != "" {
				relpath = pinfo.Linkpath
			}
			if strings.HasPrefix(relpath, "http") {
				_, err = HandleHttpCommand(w, &b, os.TempDir(), "do-echo", relpath)
				if err != nil {
					return http.StatusInternalServerError, err
				}
			} else if strings.HasPrefix(relpath, "/sources/") {
				parts := strings.Split(relpath, "/")
				if len(parts) <= 3 {
					_, err = HandleHttpCommand(w, &b, os.TempDir(), "do-log", relpath)
					if err != nil {
						return http.StatusInternalServerError, err
					}
				} else {
					_, err = HandleHttpCommand(w, &b, os.TempDir(), "do-presign", parts[2], strings.TrimRight(strings.Join(parts[3:], "/"), "/"))
					if err != nil {
						return http.StatusInternalServerError, err
					}
				}
			} else {
				_, err = HandleHttpCommand(w, &b, os.TempDir(), "do-log", relpath)
				if err != nil {
					return http.StatusInternalServerError, err
				}
			}
			abspath := strings.TrimRight(b.String(), "\n")
			if abspath == "" {
				log.Printf("invalid pointer:%v", pinfo)
				return http.StatusNoContent, nil
			}
			if r.Header.Get("Accept") == "application/json" {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				encoder := json.NewEncoder(w)
				encoder.SetEscapeHTML(false)
				err = encoder.Encode(abspath)
				if err != nil {
					return http.StatusInternalServerError, err
				}
			} else {
				setContentDisposition(w, r, file)
				w.Header().Add("Content-Security-Policy", `script-src 'none';`)
				w.Header().Set("Cache-Control", "private")
				http.Redirect(w, r, abspath, http.StatusFound)
			}
			return 0, nil
		}
	}

	fd, err := file.Fs.Open(file.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	setContentDisposition(w, r, file)
	w.Header().Add("Content-Security-Policy", `script-src 'none';`)
	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, fd)
	return 0, nil
}
