package files

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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

//nolint:gocyclo
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

func initSourceIfNecessary(fpath string) {
	if !strings.Contains(fpath, ".source") {
		return
	}

	segments := strings.Split(fpath, string(filepath.Separator))
	for i, segment := range segments {
		if strings.HasSuffix(segment, ".source") {
			sourceDirPath := filepath.Join(strings.Join(segments[:i], string(filepath.Separator)), strings.TrimSuffix(segment, ".source"))
			sourceFilePath := filepath.Join(strings.Join(segments[:i+1], string(filepath.Separator)))

			initializeSourceDirectory(sourceDirPath, sourceFilePath)
			break
		}
	}
}

func initializeSourceDirectory(sourceDirPath, sourceFilePath string) {
	if _, err := os.Stat(sourceDirPath); err == nil {
		log.Printf("Source directory already initialized: %s", sourceDirPath)
		return
	}
	log.Printf("Initializing source directory: %s", sourceDirPath)
	if err := os.MkdirAll(sourceDirPath, os.ModePerm); err != nil {
		log.Printf("Failed to create directory: %s, error: %v", sourceDirPath, err)
		return
	}
	file, err := os.Open(sourceFilePath)
	if err != nil {
		log.Printf("Failed to open .source file: %s, error: %v", sourceFilePath, err)
		return
	}
	defer file.Close()
	var dataArray []map[string]interface{}
	if err := json.NewDecoder(file).Decode(&dataArray); err != nil {
		log.Printf("Failed to decode JSON in file: %s, error: %v", sourceFilePath, err)
		return
	}
	for _, data := range dataArray {
		var name, url, checksum string
		var size int64
		for key, value := range data {
			switch key {
			case "name", "filename":
				if strVal, ok := value.(string); ok {
					name = strVal
				}
			case "size":
				if floatVal, ok := value.(float64); ok {
					size = int64(floatVal)
				}
			case "url", "download_url", "link", "href":
				if strVal, ok := value.(string); ok {
					url = strVal
				}
			case "checksum", "hash", "md5", "computed_md5":
				if strVal, ok := value.(string); ok {
					checksum = strVal
				}
			}
		}
		if url != "" && name != "" {
			symlinkPath := filepath.Join(sourceDirPath, name)
			if err := os.Symlink(url, symlinkPath); err != nil {
				log.Printf("Failed to create symbolic link for %s -> %s (%s/%d): %v", symlinkPath, url, checksum, size, err)
			}
		}
	}
}

func (pfs *PointerFs) Stat(fpath string) (os.FileInfo, error) {
	initSourceIfNecessary(fpath)
	// Stat first as strange behavior with mounted folders having IsDir flag not set with Lstat
	// Accept that invalid symbolic link will produce error
	info, err := pfs.OsFs.Stat(fpath)
	if err != nil || !info.IsDir() {
		linfo, _, lerr := pfs.OsFs.LstatIfPossible(fpath)
		if lerr != nil {
			return nil, lerr
		}
		lpath, _ := pfs.OsFs.ReadlinkIfPossible(fpath)
		if IsSymlink(linfo.Mode()) || !pfs.isExtensionWhitelisted(filepath.Ext(fpath)) || info.Size() >= pfs.Threshold {
			pinfo := &PointerInfo{
				Filename:    linfo.Name(),
				Filepath:    strings.Replace(fpath, pfs.Scope, "", 1),
				Linkpath:    strings.Replace(lpath, pfs.Scope, "", 1),
				ContentSize: linfo.Size(),
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
	return 0o644 //nolint:gomnd
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
