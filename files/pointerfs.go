package files

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"
)

var PointerFsExtensionWhitelist = []string{
	".txt",
	".csv",
	".json",
	".yml",
	".yaml",
	".xml",
	".html",
	".css",
	".js",
	".ts",
	".cs",
	".go",
	".py",
	".java",
	".markdown",
	".md",
	".stac",
	".pdf",
	".ipynb",
	".pointer",
}

type PointerFs struct {
	Scope              string
	OsFs               afero.OsFs
	RequestURI         string
	Threshold          int64
	ExtensionWhitelist []string
}

func NewPointerFs(scope string, osFs afero.OsFs, threshold int64, extensionWhitelist []string) *PointerFs {
	return &PointerFs{
		Scope:              scope,
		OsFs:               osFs,
		Threshold:          threshold,
		ExtensionWhitelist: extensionWhitelist,
	}
}

func (pfs *PointerFs) isExtensionWhitelisted(extension string) bool {
	for _, i := range pfs.ExtensionWhitelist {
		if extension == i {
			return true
		}
	}
	return false
}

func (pfs *PointerFs) OpenFile(fpath string, flag int, perm os.FileMode) (afero.File, error) {
	info, err := pfs.Stat(fpath)
	if err != nil {
		return nil, err
	}
	if pointerInfo, ok := info.(*PointerInfo); ok {
		return &Pointer{
			pointerInfo: pointerInfo,
			content:     pointerInfo.Filename + "\n" + pointerInfo.Linkpath,
		}, nil
	}
	return pfs.OsFs.OpenFile(fpath, flag, perm)
}

func (pfs *PointerFs) Open(fpath string) (afero.File, error) {
	return pfs.OpenFile(fpath, 0, 0)
}

func (pfs *PointerFs) Create(fpath string) (afero.File, error) {
	return pfs.OsFs.Create(fpath)
}

func (pfs *PointerFs) Mkdir(fpath string, perm os.FileMode) error {
	return pfs.OsFs.Mkdir(fpath, perm)
}

func (pfs *PointerFs) MkdirAll(fpath string, perm os.FileMode) error {
	return pfs.OsFs.MkdirAll(fpath, perm)
}

func (pfs *PointerFs) Remove(fpath string) error {
	return pfs.OsFs.Remove(fpath)
}

func (pfs *PointerFs) RemoveAll(fpath string) error {
	return pfs.OsFs.RemoveAll(fpath)
}

func (pfs *PointerFs) Rename(oldpath, newpath string) error {
	return pfs.OsFs.Rename(oldpath, newpath)
}

func (pfs *PointerFs) Stat(fpath string) (os.FileInfo, error) {
	if strings.HasSuffix(fpath, ".source") {
		dir := filepath.Clean(fpath)
		pinfo := &PointerInfo{
			Filename: filepath.Base(dir),
			Filepath: strings.Replace(fpath, pfs.Scope, "", 1),
		}
		return pinfo, nil
	}
	info, err := pfs.OsFs.Stat(fpath) // Stat first as strange behavior with mounted folders having IsDir flag not set with Lstat
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		linfo, _, lerr := pfs.OsFs.LstatIfPossible(fpath)
		if lerr != nil {
			return nil, lerr
		}
		lpath, _ := pfs.OsFs.ReadlinkIfPossible(fpath)
		if IsSymlink(linfo.Mode()) || !pfs.isExtensionWhitelisted(filepath.Ext(fpath)) || info.Size() >= pfs.Threshold {
			pinfo := &PointerInfo{
				Filename:    info.Name(),
				Filepath:    strings.Replace(fpath, pfs.Scope, "", 1),
				Linkpath:    strings.Replace(lpath, pfs.Scope, "", 1),
				ContentSize: info.Size(),
			}
			return pinfo, nil
		}
	}
	return info, err
}

func (pfs *PointerFs) LstatIfPossible(fpath string) (os.FileInfo, bool, error) {
	info, err := pfs.Stat(fpath)
	return info, true, err
}

func (pfs *PointerFs) Chmod(fpath string, mode os.FileMode) error {
	return pfs.OsFs.Chmod(fpath, mode)
}

func (pfs *PointerFs) Chtimes(fpath string, atime, mtime time.Time) error {
	return pfs.OsFs.Chtimes(fpath, atime, mtime)
}

func (pfs *PointerFs) Chown(fpath string, uid, gid int) error {
	return pfs.OsFs.Chown(fpath, uid, gid)
}

func (pfs *PointerFs) Name() string {
	return fmt.Sprintf("PointerFs (threshold: %d bytes)", pfs.Threshold)
}

func (pfs *PointerFs) SymlinkIfPossible(oldname, newname string) error {
	return pfs.OsFs.SymlinkIfPossible(oldname, newname)
}

type Pointer struct {
	pointerInfo *PointerInfo
	children    []*PointerInfo
	content     string
	offset      int64
	closed      bool
}

func NewPointer(pointerInfo *PointerInfo, content string) *Pointer {
	return &Pointer{
		pointerInfo: pointerInfo,
		content:     content,
	}
}

// //noling:goconst
// func (p *Pointer) initChildren() {
// 	if p.children == nil {
// 		file, _ := p.osFs.Open(p.pointerInfo.Filepath)

// 		var dataArray []map[string]interface{}
// 		decoder := json.NewDecoder(file)
// 		if err := decoder.Decode(&dataArray); err != nil {
// 			fmt.Println("Error decoding JSON:", err)
// 			return
// 		}

// 		for _, data := range dataArray {
// 			var name, url, checksum string
// 			var size int64

// 			for key, value := range data {
// 				switch key {
// 				case "name", "filename":
// 					if strVal, ok := value.(string); ok {
// 						name = strVal
// 					}
// 				case "size":
// 					if intVal, ok := value.(float64); ok { // JSON numbers decode as float64
// 						size = int64(intVal)
// 					}
// 				case "url", "download_url", "link", "href":
// 					if strVal, ok := value.(string); ok {
// 						url = strVal
// 					}
// 				case "checksum", "hash", "md5", "computed_md5":
// 					if strVal, ok := value.(string); ok {
// 						checksum = strVal
// 					}
// 				}
// 			}

// 			if name != "" {
// 				pinfo := &PointerInfo{
// 					Filename:    name,
// 					Filepath:    path.Join(p.pointerInfo.Filepath, name),
// 					Linkpath:    url,
// 					ContentSize: size,
// 					Checksum:    checksum,
// 				}
// 				p.children = append(p.children, pinfo)
// 			}
// 		}
// 	}
// }

func (p *Pointer) Read(b []byte) (int, error) {
	if p.closed {
		return 0, io.EOF
	}
	if p.offset >= int64(len(p.content)) {
		return 0, io.EOF
	}
	n := copy(b, p.content[p.offset:])
	p.offset += int64(n)
	return n, nil
}

func (p *Pointer) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = p.offset + offset
	case io.SeekEnd:
		newOffset = int64(len(p.content)) + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	if newOffset < 0 || newOffset > int64(len(p.content)) {
		return 0, fmt.Errorf("invalid seek offset")
	}
	p.offset = newOffset
	return p.offset, nil
}

func (p *Pointer) Close() error {
	p.closed = true
	return nil
}

func (p *Pointer) Name() string {
	return p.pointerInfo.Name()
}

//nolint:revive
func (p *Pointer) Write(b []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (p *Pointer) Sync() error {
	return nil
}

//nolint:revive
func (p *Pointer) Truncate(size int64) error {
	return nil
}

//nolint:revive
func (p *Pointer) WriteAt(b []byte, off int64) (int, error) {
	return 0, io.ErrClosedPipe
}

//nolint:revive
func (p *Pointer) ReadAt(b []byte, off int64) (int, error) {
	return 0, io.EOF
}

//nolint:revive
func (p *Pointer) Readdir(count int) ([]os.FileInfo, error) {
	if len(p.children) == 0 {
		return nil, io.EOF
	}

	// count <= 0 return all
	if count <= 0 || count > len(p.children) {
		count = len(p.children)
	}

	var fileInfos []os.FileInfo
	for i := 0; i < count; i++ {
		child := p.children[i]
		childp := Pointer{pointerInfo: child, content: path.Join(p.content, child.Filename)}
		info, err := childp.Stat()
		if err != nil {
			return nil, err
		}
		fileInfos = append(fileInfos, info)
	}

	return fileInfos, nil
}

//nolint:revive
func (p *Pointer) Readdirnames(n int) ([]string, error) {
	if len(p.children) == 0 {
		return nil, io.EOF
	}

	// count <= 0 return all
	if n <= 0 || n > len(p.children) {
		n = len(p.children)
	}

	names := make([]string, n)
	for i := 0; i < n; i++ {
		names[i] = p.children[i].Name()
	}

	return names, nil
}

func (p *Pointer) Stat() (os.FileInfo, error) {
	return p.pointerInfo, nil
}

//nolint:revive
func (p *Pointer) WriteString(s string) (int, error) {
	return 0, io.ErrClosedPipe
}

type PointerInfo struct {
	Filename    string
	Filepath    string // relative
	Linkpath    string // relative
	ContentSize int64
	Checksum    string
}

func (pi *PointerInfo) Name() string {
	return pi.Filename
}

func (pi *PointerInfo) Size() int64 {
	return pi.ContentSize
}

func (pi *PointerInfo) Mode() os.FileMode {
	return 0o644 //nolint:gomod
}

func (pi *PointerInfo) ModTime() time.Time {
	return time.Time{}
}

func (pi *PointerInfo) IsDir() bool {
	return strings.HasSuffix(pi.Filepath, "/")
}

func (pi *PointerInfo) Sys() interface{} {
	return nil
}
