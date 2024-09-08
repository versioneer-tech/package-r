package files

import (
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/v2/rules"
	"github.com/versioneer-tech/package-r/v2/share"
)

const PermFile = 0644
const PermDir = 0755

// FileInfo describes a file.
type FileInfo struct {
	*Listing
	Fs        afero.Fs          `json:"-"`
	Path      string            `json:"path"`
	Source    share.Source      `json:"source"`
	Name      string            `json:"name"`
	Size      int64             `json:"size"`
	Extension string            `json:"extension"`
	ModTime   time.Time         `json:"modified"`
	Mode      os.FileMode       `json:"mode"`
	IsDir     bool              `json:"isDir"`
	IsSymlink bool              `json:"isSymlink"`
	Type      string            `json:"type"`
	Content   string            `json:"content,omitempty"`
	Checksums map[string]string `json:"checksums,omitempty"`
	Token     string            `json:"token,omitempty"`
}

// FileOptions are the options when getting a file info.
type FileOptions struct {
	Fs      afero.Fs
	Path    string
	Source  share.Source
	Modify  bool
	Expand  bool
	Token   string
	Checker rules.Checker
	Content bool
}

type Presigner interface {
	Presign(keys []string) (presignedUrls []string, status int, err error)
}

// NewFileInfo creates a File object from a path and a given user. This File
// object will be automatically filled depending on if it is a directory
// or a file.
func NewFileInfo(opts *FileOptions) (*FileInfo, error) {
	if !opts.Checker.Check(opts.Path) {
		return nil, os.ErrPermission
	}

	file, err := stat(opts)
	if err != nil {
		return nil, err
	}

	if opts.Expand {
		if file.IsDir {
			if err := file.readListing(opts.Checker); err != nil { //nolint:govet
				return nil, err
			}
			return file, nil
		}

		err = file.detectType(opts.Modify, opts.Content)
		if err != nil {
			return nil, err
		}
	}

	return file, err
}

func stat(opts *FileOptions) (*FileInfo, error) {
	var file *FileInfo

	if lstaterFs, ok := opts.Fs.(afero.Lstater); ok {
		info, _, err := lstaterFs.LstatIfPossible(opts.Path)
		if err != nil {
			return nil, err
		}
		file = &FileInfo{
			Fs:        opts.Fs,
			Path:      opts.Path,
			Source:    opts.Source,
			Name:      info.Name(),
			ModTime:   info.ModTime(),
			Mode:      info.Mode(),
			IsDir:     info.IsDir(),
			IsSymlink: IsSymlink(info.Mode()),
			Size:      info.Size(),
			Extension: filepath.Ext(info.Name()),
			Token:     opts.Token,
		}
	}

	// regular file
	if file != nil && !file.IsSymlink {
		return file, nil
	}

	// fs doesn't support afero.Lstater interface or the file is a symlink
	info, err := opts.Fs.Stat(opts.Path)
	if err != nil {
		// can't follow symlink
		if file != nil && file.IsSymlink {
			return file, nil
		}
		return nil, err
	}

	// set correct file size in case of symlink
	if file != nil && file.IsSymlink {
		file.Size = info.Size()
		file.IsDir = info.IsDir()
		return file, nil
	}

	file = &FileInfo{
		Fs:        opts.Fs,
		Path:      opts.Path,
		Source:    opts.Source,
		Name:      info.Name(),
		ModTime:   info.ModTime(),
		Mode:      info.Mode(),
		IsDir:     info.IsDir(),
		Size:      info.Size(),
		Extension: filepath.Ext(info.Name()),
		Token:     opts.Token,
	}

	return file, nil
}

// // Checksum checksums a given File for a given User, using a specific
// // algorithm. The checksums data is saved on File object.
// func (i *FileInfo) Checksum(algo string) error {
// 	if i.IsDir {
// 		return fbErrors.ErrIsDirectory
// 	}

// 	if i.Checksums == nil {
// 		i.Checksums = map[string]string{}
// 	}

// 	reader, err := i.Fs.Open(i.Path)
// 	if err != nil {
// 		return err
// 	}
// 	defer reader.Close()

// 	var h hash.Hash

// 	//nolint:gosec
// 	switch algo {
// 	case "md5":
// 		h = md5.New()
// 	case "sha1":
// 		h = sha1.New()
// 	case "sha256":
// 		h = sha256.New()
// 	case "sha512":
// 		h = sha512.New()
// 	default:
// 		return fbErrors.ErrInvalidOption
// 	}

// 	_, err = io.Copy(h, reader)
// 	if err != nil {
// 		return err
// 	}

// 	i.Checksums[algo] = hex.EncodeToString(h.Sum(nil))
// 	return nil
// }

func (i *FileInfo) RealPath() string {
	if realPathFs, ok := i.Fs.(interface {
		RealPath(name string) (fPath string, err error)
	}); ok {
		realPath, err := realPathFs.RealPath(i.Path)
		if err == nil {
			return realPath
		}
	}

	return i.Path
}

//nolint:goconst
func (i *FileInfo) detectType(modify, saveContent bool) error {
	if IsNamedPipe(i.Mode) {
		i.Type = "blob"
		return nil
	}
	// failing to detect the type should not return error.
	// imagine the situation where a file in a dir with thousands
	// of files couldn't be opened: we'd have immediately
	// a 500 even though it doesn't matter. So we just log it.

	mimetype := mime.TypeByExtension(i.Extension)

	var buffer []byte

	switch {
	case strings.HasPrefix(mimetype, "video"):
		i.Type = "video"
		return nil
	case strings.HasPrefix(mimetype, "audio"):
		i.Type = "audio"
		return nil
	case strings.HasPrefix(mimetype, "image"):
		i.Type = "image"
		return nil
	case strings.HasSuffix(mimetype, "pdf"):
		i.Type = "pdf"
		return nil
	case (strings.HasPrefix(mimetype, "text") || !isBinary(buffer)) && i.Size <= 10*1024*1024: // 10 MB
		i.Type = "text"

		if !modify {
			i.Type = "textImmutable"
		}

		if saveContent {
			afs := &afero.Afero{Fs: i.Fs}
			content, err := afs.ReadFile(i.Path)
			if err != nil {
				return err
			}

			i.Content = string(content)
		}
		return nil
	default:
		i.Type = "blob"
	}

	return nil
}

func (i *FileInfo) readListing(checker rules.Checker) error {
	afs := &afero.Afero{Fs: i.Fs}
	dir, err := afs.ReadDir(i.Path)
	if err != nil {
		return err
	}

	listing := &Listing{
		Items:    []*FileInfo{},
		NumDirs:  0,
		NumFiles: 0,
	}

	for _, f := range dir {
		name := f.Name()
		fPath := path.Join(i.Path, name)

		if !checker.Check(fPath) {
			continue
		}

		isSymlink, isInvalidLink := false, false
		if IsSymlink(f.Mode()) {
			isSymlink = true
			// It's a symbolic link. We try to follow it. If it doesn't work,
			// we stay with the link information instead of the target's.
			info, err := i.Fs.Stat(fPath)
			if err == nil {
				f = info
			} else {
				isInvalidLink = true
			}
		}

		file := &FileInfo{
			Fs:        i.Fs,
			Name:      name,
			Size:      f.Size(),
			ModTime:   f.ModTime(),
			Mode:      f.Mode(),
			IsDir:     f.IsDir(),
			IsSymlink: isSymlink,
			Extension: filepath.Ext(name),
			Path:      fPath,
			Source:    i.Source,
		}

		if file.IsDir {
			listing.NumDirs++
		} else {
			listing.NumFiles++

			if isInvalidLink {
				file.Type = "invalid_link"
			} else {
				err := file.detectType(true, false)
				if err != nil {
					return err
				}
			}
		}

		listing.Items = append(listing.Items, file)
	}

	i.Listing = listing
	return nil
}
