package http

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	gopath "path"
	"path/filepath"
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

//nolint:goconst
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

//nolint:goconst
func setContentDisposition(w http.ResponseWriter, r *http.Request, finfo *files.FileInfo) {
	if r.URL.Query().Get("inline") == "true" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		// As per RFC6266 section 4.3
		filename := finfo.Name
		if finfo.Type == "pointer" {
			filename += ".pointer"
		}
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(filename))
	}
}

var rawHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Download {
		return http.StatusAccepted, nil
	}

	finfo, err := files.NewFileInfo(&files.FileOptions{
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

	if files.IsNamedPipe(finfo.Mode) {
		setContentDisposition(w, r, finfo)
		return 0, nil
	}

	if !finfo.IsDir {
		return rawFileHandler(w, r, finfo)
	}

	return rawDirHandler(w, r, d, finfo)
})

func addFile(ar archiver.Writer, d *data, fpath, commonPath string) error {
	if !d.Check(fpath) {
		return nil
	}

	finfo, err := d.user.Fs.Stat(fpath)
	if err != nil {
		return err
	}

	if !finfo.IsDir() && !finfo.Mode().IsRegular() {
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
		if _, ok := finfo.(*files.PointerInfo); ok {
			filename += ".pointer"
		}
		err = ar.Write(archiver.File{
			FileInfo: archiver.FileInfo{
				FileInfo:   finfo,
				CustomName: filename,
			},
			ReadCloser: file,
		})
		if err != nil {
			return err
		}
	}

	if finfo.IsDir() {
		names, err := file.Readdirnames(0)
		if err != nil {
			return err
		}

		for _, name := range names {
			fPath := filepath.Join(fpath, name)
			err = addFile(ar, d, fPath, commonPath)
			if err != nil {
				log.Printf("Failed to archive %s: %v", fPath, err)
			}
		}
	}

	return nil
}

func rawDirHandler(w http.ResponseWriter, r *http.Request, d *data, finfo *files.FileInfo) (int, error) {
	filenames, err := parseQueryFiles(r, finfo, d.user)
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
	if !finfo.IsDir {
		commonDir = filepath.Dir(finfo.Path)
	} else {
		commonDir = fileutils.CommonPrefix(filepath.Separator, filenames...)
	}

	name := filepath.Base(commonDir)
	if name == "." || name == "" || name == string(filepath.Separator) {
		name = finfo.Name
	}
	// Prefix used to distinguish a filelist generated
	// archive from the full directory archive
	if len(filenames) > 1 {
		name = "_" + name
	}
	name += extension
	w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(name))

	for _, fname := range filenames {
		err = addFile(ar, d, fname, commonDir)
		if err != nil {
			log.Printf("Failed to archive %s: %v", fname, err)
		}
	}

	return 0, nil
}

func rawFileHandler(w http.ResponseWriter, r *http.Request, finfo *files.FileInfo) (int, error) {
	file, err := finfo.Fs.Open(finfo.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer file.Close()

	if finfo.Type == "pointer" && (strings.HasSuffix(r.URL.Path, ".pointer") || r.URL.Query().Get("inline") == "true") {
		buffer := make([]byte, 512) //nolint:gomnd
		n, _ := file.Read(buffer)
		http.Redirect(w, r, string(buffer[:n]), http.StatusFound)
	} else {
		setContentDisposition(w, r, finfo)
		w.Header().Add("Content-Security-Policy", `script-src 'none';`)
		w.Header().Set("Cache-Control", "private")
		http.ServeContent(w, r, finfo.Name, finfo.ModTime, file)
	}
	return 0, nil
}
