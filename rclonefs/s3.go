package rclonefs

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/backend/s3"
	rclone "github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/objectstorage"
)

// ErrManagerClosed reports use of a manager after shutdown.
var ErrManagerClosed = errors.New("rclone filesystem manager is closed")

// Manager owns and reuses one rclone VFS for each distinct S3 configuration.
type Manager struct {
	ctx        context.Context
	root       string
	buckets    string
	setBuckets bool
	mux        sync.Mutex
	files      map[objectstorage.Config]*FS
	views      map[objectstorage.Config]afero.Fs
	nextID     uint64
	closed     bool
}

// NewManager creates a process-scoped filesystem manager.
func NewManager(ctx context.Context, root string, buckets ...string) *Manager {
	manager := &Manager{
		ctx:   ctx,
		root:  root,
		files: make(map[objectstorage.Config]*FS),
		views: make(map[objectstorage.Config]afero.Fs),
	}
	if len(buckets) > 0 {
		manager.buckets = buckets[0]
		manager.setBuckets = true
	}
	return manager
}

// FileSystem returns the process-owned shared filesystem.
func (m *Manager) FileSystem() (afero.Fs, error) {
	return m.fileSystem()
}

// PublicLink creates a time-limited read URL through the same backend as the VFS.
func (m *Manager) PublicLink(ctx context.Context, name string, expire time.Duration) (string, error) {
	fileSystem, err := m.fileSystem()
	if err != nil {
		return "", err
	}
	if catalog, ok := fileSystem.(*bucketCatalogFS); ok {
		backend, child, exact, err := catalog.resolve("public link", name)
		if err != nil {
			return "", err
		}
		if exact {
			return "", pathError("public link", name, errors.New("bucket roots do not have public links"))
		}
		return backend.(*FS).PublicLink(ctx, child, expire)
	}
	return fileSystem.(*FS).PublicLink(ctx, name, expire)
}

// Stats returns aggregate values for all VFS instances owned by the manager.
func (m *Manager) Stats() Stats {
	m.mux.Lock()
	defer m.mux.Unlock()

	var total Stats
	for _, fileSystem := range m.files {
		stats := fileSystem.stats()
		total.CacheBytes += stats.CacheBytes
		total.ErroredFiles += stats.ErroredFiles
		total.UploadsInProgress += stats.UploadsInProgress
		total.UploadsQueued += stats.UploadsQueued
		total.OutOfSpace = total.OutOfSpace || stats.OutOfSpace
	}
	return total
}

func (m *Manager) fileSystem() (afero.Fs, error) {
	config := objectstorage.Load()
	config.SetRoot(m.root)
	if m.setBuckets {
		config.SetBuckets(m.buckets)
	}
	if err := config.ValidateFilesystem(); err != nil {
		return nil, err
	}

	m.mux.Lock()
	defer m.mux.Unlock()
	if m.closed {
		return nil, ErrManagerClosed
	}
	if view, ok := m.views[config]; ok {
		return view, nil
	}

	if buckets := config.ConfiguredBuckets(); len(buckets) > 0 {
		backends := make(map[string]afero.Fs, len(buckets))
		for _, bucket := range buckets {
			bucketConfig := config
			bucketConfig.SetBuckets("")
			bucketConfig.SetRoot(bucket)
			fileSystem, err := m.fileSystemLocked(bucketConfig)
			if err != nil {
				return nil, err
			}
			backends[bucket] = fileSystem
		}
		catalog, err := newBucketCatalogFS(backends)
		if err != nil {
			return nil, fmt.Errorf("create configured bucket catalog: %w", err)
		}
		m.views[config] = catalog
		return catalog, nil
	}

	fileSystem, err := m.fileSystemLocked(config)
	if err != nil {
		return nil, err
	}
	m.views[config] = fileSystem
	return fileSystem, nil
}

func (m *Manager) fileSystemLocked(config objectstorage.Config) (*FS, error) {
	if fileSystem, ok := m.files[config]; ok {
		return fileSystem, nil
	}
	m.nextID++
	name := fmt.Sprintf("object-%d", m.nextID)
	fileSystem, err := NewS3(m.ctx, name, config)
	if err != nil {
		return nil, err
	}
	m.files[config] = fileSystem
	return fileSystem, nil
}

// Close stops all VFS instances owned by the manager.
func (m *Manager) Close() error {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	for _, fileSystem := range m.files {
		_ = fileSystem.Close()
	}
	clear(m.files)
	clear(m.views)
	return nil
}

// NewS3 creates an afero adapter at the S3 service root or one bucket.
func NewS3(ctx context.Context, name string, config objectstorage.Config) (*FS, error) {
	if err := config.ValidateFilesystem(); err != nil {
		return nil, err
	}

	ctx, err := newRcloneContext(ctx)
	if err != nil {
		return nil, err
	}
	values := s3ConfigValues(config)

	info, err := rclone.Find("s3")
	if err != nil {
		return nil, fmt.Errorf("find rclone S3 backend: %w", err)
	}
	mapper := configmap.New().
		AddGetter(values, configmap.PriorityNormal).
		AddGetter(optionDefaults{options: info.Options}, configmap.PriorityDefault)
	remote, err := s3.NewFs(ctx, name, config.Root(), mapper)
	if err != nil {
		return nil, fmt.Errorf("create rclone S3 backend: %w", err)
	}
	return New(ctx, remote)
}

func newRcloneContext(ctx context.Context) (context.Context, error) {
	ctx, config := rclone.AddConfig(ctx)
	if err := configstruct.Set(optionDefaults{options: rclone.ConfigOptionsInfo}, config); err != nil {
		return nil, fmt.Errorf("configure rclone defaults: %w", err)
	}
	// S3 listings already include LastModified. Use it instead of reading
	// object metadata once for every listed file.
	config.UseServerModTime = true
	return ctx, nil
}

func s3ConfigValues(config objectstorage.Config) configmap.Simple {
	provider := "AWS"
	if config.Endpoint != "" {
		provider = "Other"
	}
	useAmbientCredentials := config.UsesAmbientCredentials()
	accessKeyID := config.AccessKeyID
	secretAccessKey := config.SecretAccessKey
	sessionToken := config.SessionToken
	if useAmbientCredentials {
		accessKeyID = ""
		secretAccessKey = ""
		sessionToken = ""
	}

	return configmap.Simple{
		"provider":          provider,
		"env_auth":          strconv.FormatBool(useAmbientCredentials),
		"access_key_id":     accessKeyID,
		"secret_access_key": secretAccessKey,
		"session_token":     sessionToken,
		"endpoint":          config.Endpoint,
		"region":            config.Region,
		"force_path_style":  "true",
		"no_check_bucket":   "true",
		"no_head_object":    "true",
	}
}

// optionDefaults keeps unrelated RCLONE_* environment variables out of the
// embedded backend while retaining rclone's registered S3 defaults.
type optionDefaults struct {
	options rclone.Options
}

func (d optionDefaults) Get(key string) (string, bool) {
	option := d.options.Get(key)
	if option == nil {
		return "", false
	}
	copy := option.Copy()
	copy.Value = nil
	return copy.String(), true
}
