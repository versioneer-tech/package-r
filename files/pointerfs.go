package files

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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
	".source",
}

type PointerFs struct {
	Scope              string
	OsFs               afero.OsFs
	Threshold          int64
	ExtensionWhitelist []string
	S3s                map[string]*S3 // per source
}

func NewPointerFs(scope string, osFs afero.OsFs, threshold int64, extensionWhitelist []string) *PointerFs {
	return &PointerFs{
		Scope:              scope,
		OsFs:               osFs,
		Threshold:          threshold,
		ExtensionWhitelist: extensionWhitelist,
		S3s:                make(map[string]*S3),
	}
}

func getStringOrDefault(values map[string]string, key, defaultValue string) string {
	if value, ok := values[key]; ok && value != "" {
		return value
	}
	return defaultValue
}

func connect(source string) (s3Client *s3.S3, bucketName, bucketPrefix string, err error) {
	values := make(map[string]string)
	keys := []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION", "AWS_ENDPOINT_URL", "BUCKET_NAME", "BUCKET_PREFIX"}
	for _, key := range keys {
		filePath := "/secrets/" + filepath.Join(source, key)
		data, _ := os.ReadFile(filePath)
		if data != nil {
			values[key] = strings.TrimSpace(string(data))
		}
	}
	sess, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			getStringOrDefault(values, "AWS_ACCESS_KEY_ID", os.Getenv("AWS_ACCESS_KEY_ID")),
			getStringOrDefault(values, "AWS_SECRET_ACCESS_KEY", os.Getenv("AWS_SECRET_ACCESS_KEY")),
			"",
		),
		Endpoint:         aws.String(getStringOrDefault(values, "AWS_ENDPOINT_URL", os.Getenv("AWS_ENDPOINT_URL"))),
		Region:           aws.String(getStringOrDefault(values, "AWS_REGION", os.Getenv("AWS_REGION"))),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return nil, "", "", fmt.Errorf("could not create AWS session: %w", err)
	}
	return s3.New(sess), getStringOrDefault(values, "BUCKET_NAME", source), getStringOrDefault(values, "BUCKET_PREFIX", ""), nil
}

func (pfs *PointerFs) withS3(source string) *S3 {
	if _, exists := pfs.S3s[source]; !exists {
		s3Client, bucketName, bucketPrefix, err := connect(source)
		log.Printf("withS3 for %s:%v", bucketName, err)
		pfs.S3s[source] = &S3{
			s3Client:     s3Client,
			bucketName:   bucketName,
			bucketPrefix: bucketPrefix,
		}
	}
	return pfs.S3s[source]
}

type S3 struct {
	s3Client     *s3.S3
	bucketName   string
	bucketPrefix string
}

//nolint:gocritic
func (p *S3) presign(path string) (string, error) {
	if p == nil || p.s3Client == nil {
		return "", fmt.Errorf("presign without S3 not possible using %s with %s", p.bucketName, path)
	}
	if path == "" {
		return "", fmt.Errorf("presign with empty path not possible using %s", p.bucketName)
	}
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, p.bucketPrefix)
	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(p.bucketName),
		Key:    aws.String(path),
	}
	req, _ := p.s3Client.GetObjectRequest(getObjectInput)
	presignedURL, err := req.Presign(7 * 24 * time.Hour)
	if err != nil {
		return "", fmt.Errorf("could not presign object %s: %w", path, err)
	}
	return presignedURL, nil
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
		if flag&os.O_CREATE == os.O_CREATE {
			return pfs.OsFs.OpenFile(fpath, flag, perm)
		}
		return nil, err
	}
	if pinfo, ok := info.(*PointerInfo); ok {
		relpath := pinfo.linkpath
		if relpath == "" {
			relpath = pinfo.filepath
		}
		if strings.HasPrefix(relpath, "/sources/") {
			parts := strings.Split(relpath, "/")
			if len(parts) > 3 {
				source := parts[2]
				if source != "" {
					url, err := pfs.withS3(source).presign(strings.TrimRight(strings.Join(parts[3:], "/"), "/"))
					log.Printf("presign %s -> %s:%v", relpath, url, err)
					return &Pointer{
						URL:   url,
						pinfo: pinfo,
					}, nil
				}
			}
		}
		return &Pointer{
			URL:   relpath,
			pinfo: pinfo,
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
			sourceFilePath := strings.Join(segments[:i+1], string(filepath.Separator))

			if err := initializeSourceDirectory(sourceDirPath, sourceFilePath); err != nil {
				log.Printf("Error initializing source directory: %v", err)
			}
			break
		}
	}
}

var dirCreationMutex sync.Mutex

//nolint:gomnd
func createSymlink(url, abspath string) error {
	if strings.HasSuffix(abspath, "/") {
		return nil
	}

	dirPath := filepath.Dir(abspath)

	dirCreationMutex.Lock()
	defer dirCreationMutex.Unlock()

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directories for %s: %w", dirPath, err)
	}

	if err := os.Symlink(url, abspath); err != nil {
		if os.IsExist(err) {
			log.Printf("Symlink already exists for %s, skipping", abspath)
			return nil
		}
		return fmt.Errorf("failed to create symlink for %s: %w", abspath, err)
	}

	return nil
}

//nolint:funlen,gocyclo
func initializeSourceDirectory(sourceDirPath, sourceFilePath string) error {
	if _, err := os.Stat(sourceDirPath); err == nil {
		return nil
	}

	log.Printf("Initializing source directory: %s", sourceDirPath)
	if err := os.MkdirAll(sourceDirPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %s, error: %w", sourceDirPath, err)
	}

	file, err := os.Open(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var createdLinks int64
	scanner := bufio.NewScanner(file)
	var firstLine string
	if scanner.Scan() {
		firstLine = strings.TrimSpace(scanner.Text())
	}
	if strings.HasPrefix(firstLine, "[") {
		file.Seek(0, 0) //nolint:errcheck
		var dataArray []map[string]interface{}
		if err := json.NewDecoder(file).Decode(&dataArray); err != nil {
			return fmt.Errorf("failed to decode JSON in file: %s, error: %w", sourceFilePath, err)
		}
		for _, data := range dataArray {
			var name, url string
			for key, value := range data {
				switch key {
				case "name", "filename":
					if strVal, ok := value.(string); ok {
						name = strVal
					}
				case "url", "download_url", "link", "href":
					if strVal, ok := value.(string); ok {
						url = strVal
					}
				}
			}
			if url != "" {
				if name == "" {
					if idx := strings.Index(url, "://"); idx != -1 {
						name = url[idx+3:]
					} else {
						name = url
					}
				}
				if err := createSymlink(url, path.Join(sourceDirPath, name)); err != nil {
					log.Printf("Failed to create symlink for %s: %v", name, err)
				} else {
					atomic.AddInt64(&createdLinks, 1)
				}
			}
		}
		log.Printf("Total symlinks created: %d", createdLinks)
		return nil
	}

	file.Seek(0, 0) //nolint:errcheck
	scanner = bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			var name, url string
			if idx := strings.Index(line, "://"); idx != -1 {
				name = line[idx+3:]
				url = line
			} else {
				name = line
				url = line
			}
			if err := createSymlink(url, path.Join(sourceDirPath, name)); err != nil {
				log.Printf("Failed to create symlink for %s: %v", name, err)
			} else {
				atomic.AddInt64(&createdLinks, 1)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	log.Printf("Total symlinks created: %d", createdLinks)
	return nil
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
				filename:    linfo.Name(),
				filepath:    strings.Replace(fpath, pfs.Scope, "", 1),
				linkpath:    strings.Replace(lpath, pfs.Scope, "", 1),
				contentSize: linfo.Size(),
			}
			return pinfo, nil
		}
	}
	return info, err
}

//nolint:gocritic
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
	URL      string
	pinfo    *PointerInfo
	children []*PointerInfo
	offset   int64
	closed   bool
}

func (p *Pointer) Read(b []byte) (int, error) {
	if p.closed {
		return 0, io.EOF
	}
	if p.offset >= int64(len(p.URL)) {
		return 0, io.EOF
	}
	n := copy(b, p.URL[p.offset:])
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
		newOffset = int64(len(p.URL)) + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	if newOffset < 0 || newOffset > int64(len(p.URL)) {
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
	return p.pinfo.Name()
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
		childp := Pointer{pinfo: child, URL: path.Join(p.URL, child.filename)}
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
	return p.pinfo, nil
}

//nolint:revive
func (p *Pointer) WriteString(s string) (int, error) {
	return 0, io.ErrClosedPipe
}

type PointerInfo struct {
	filename    string
	filepath    string // relative
	linkpath    string // relative
	contentSize int64
	// checksum    string
}

func (pi *PointerInfo) Name() string {
	return pi.filename
}

func (pi *PointerInfo) Size() int64 {
	return pi.contentSize
}

func (pi *PointerInfo) Mode() os.FileMode {
	return 0o644 //nolint:gomnd
}

func (pi *PointerInfo) ModTime() time.Time {
	return time.Time{}
}

func (pi *PointerInfo) IsDir() bool {
	return strings.HasSuffix(pi.filepath, "/")
}

func (pi *PointerInfo) Sys() interface{} {
	return nil
}
