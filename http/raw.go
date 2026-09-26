package http

import (
	"archive/zip"
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	gopath "path"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"

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
			name, err := url.QueryUnescape(strings.ReplaceAll(name, "+", "%2B"))
			if err != nil {
				return nil, err
			}

			name = slashClean(name)
			fileSlice = append(fileSlice, filepath.Join(f.Path, name))
		}
	}

	return fileSlice, nil
}

func parseQueryAlgorithm(r *http.Request) (string, archives.Archiver, error) {
	switch r.URL.Query().Get("algo") {
	case "zip", "true", "":
		return ".zip", archives.Zip{Compression: zip.Deflate, SelectiveCompression: true}, nil
	case "tar":
		return ".tar", archives.Tar{}, nil
	case "targz":
		return ".tar.gz", archives.CompressedArchive{Compression: archives.Gz{}, Archival: archives.Tar{}}, nil
	case "tarbz2":
		return ".tar.bz2", archives.CompressedArchive{Compression: archives.Bz2{}, Archival: archives.Tar{}}, nil
	case "tarxz":
		return ".tar.xz", archives.CompressedArchive{Compression: archives.Xz{}, Archival: archives.Tar{}}, nil
	case "tarlz4":
		return ".tar.lz4", archives.CompressedArchive{Compression: archives.Lz4{}, Archival: archives.Tar{}}, nil
	case "tarsz":
		return ".tar.sz", archives.CompressedArchive{
			Compression: archives.Sz{S2: archives.S2{Compression: archives.S2LevelFast}},
			Archival:    archives.Tar{},
		}, nil
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

var rawHandler = withUser(rawGetHandler)

func rawGetHandler(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Download {
		return http.StatusForbidden, nil
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
		log.Printf("[DOWNLOAD] user=%q path=%q delivery=package_r", d.user.Username, file.Path)
		setContentDisposition(w, r, file)
		return 0, nil
	}

	log.Printf("[DOWNLOAD] user=%q path=%q delivery=package_r", d.user.Username, file.Path)
	if !file.IsDir {
		return rawFileHandler(w, r, file)
	}

	return rawDirHandler(w, r, d, file)
}

func collectArchiveFiles(ctx context.Context, d *data, path, commonPath string, entries *[]archives.FileInfo) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !d.Check(path) {
		return nil
	}

	info, err := d.user.Fs.Stat(path)
	if err != nil {
		return err
	}

	if !info.IsDir() && !info.Mode().IsRegular() {
		return nil
	}

	if path != commonPath {
		filename, err := filepath.Rel(commonPath, path)
		if err != nil {
			return err
		}
		if !filepath.IsLocal(filename) {
			return errors.New("archive path is outside the selected directory")
		}
		*entries = append(*entries, archives.FileInfo{
			FileInfo:      info,
			NameInArchive: filepath.ToSlash(filename),
			Open:          func() (fs.File, error) { return d.user.Fs.Open(path) },
		})
	}

	if info.IsDir() {
		file, err := d.user.Fs.Open(path)
		if err != nil {
			return err
		}
		names, err := file.Readdirnames(0)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}

		for _, name := range names {
			fPath := filepath.Join(path, name)
			err = collectArchiveFiles(ctx, d, fPath, commonPath, entries)
			if err != nil {
				return err
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

	commonDir := fileutils.CommonPrefix(filepath.Separator, filenames...)
	var entries []archives.FileInfo
	for _, fname := range filenames {
		if err := collectArchiveFiles(r.Context(), d, fname, commonDir, &entries); err != nil {
			return http.StatusInternalServerError, err
		}
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

	if err := ar.Archive(r.Context(), w, entries); err != nil {
		return http.StatusInternalServerError, err
	}

	return 0, nil
}

func rawFileHandler(w http.ResponseWriter, r *http.Request, file *files.FileInfo) (int, error) {
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
