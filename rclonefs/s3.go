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
	ctx    context.Context
	root   string
	mux    sync.Mutex
	files  map[objectstorage.Config]*FS
	nextID uint64
	closed bool
}

// NewManager creates a process-scoped filesystem manager.
func NewManager(ctx context.Context, root string) *Manager {
	return &Manager{
		ctx:   ctx,
		root:  root,
		files: make(map[objectstorage.Config]*FS),
	}
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
	return fileSystem.PublicLink(ctx, name, expire)
}

func (m *Manager) fileSystem() (*FS, error) {
	config := objectstorage.Load()
	config.SetRoot(m.root)
	if err := config.ValidateFilesystem(); err != nil {
		return nil, err
	}

	m.mux.Lock()
	defer m.mux.Unlock()
	if m.closed {
		return nil, ErrManagerClosed
	}
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
	return nil
}

// NewS3 creates an afero adapter at the S3 service root or one bucket.
func NewS3(ctx context.Context, name string, config objectstorage.Config) (*FS, error) {
	if err := config.ValidateFilesystem(); err != nil {
		return nil, err
	}

	ctx, globalConfig := rclone.AddConfig(ctx)
	if err := configstruct.Set(optionDefaults{options: rclone.ConfigOptionsInfo}, globalConfig); err != nil {
		return nil, fmt.Errorf("configure rclone defaults: %w", err)
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
