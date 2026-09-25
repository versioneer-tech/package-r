package objectstorage

import (
	"errors"
	"os"
	"path"
	"strings"
)

var (
	// ErrMissingCredentials reports an incomplete static credential pair.
	ErrMissingCredentials = errors.New("set both AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY, or neither to use ambient AWS credentials")
	// ErrInvalidRoot reports an FB_ROOT value that is not / or one bucket name.
	ErrInvalidRoot = errors.New("FB_ROOT must be / or one S3 bucket name without /")
	// ErrUserDirNeedsBucket reports a generated user directory without one bucket.
	ErrUserDirNeedsBucket = errors.New("FB_CREATE_USER_DIR=true requires FB_ROOT to name one S3 bucket; FB_ROOT=/ exposes the S3 service root")
	// ErrInvalidObjectPath reports a path outside the user's storage scope.
	ErrInvalidObjectPath = errors.New("invalid object path")
)

// Config contains the S3 settings shared by filesystem access and presigning.
type Config struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Endpoint        string
	Region          string
	Bucket          string
}

// Load reads the process-owned S3 connection settings.
func Load() Config {
	return Config{
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		SessionToken:    os.Getenv("AWS_SESSION_TOKEN"),
		Endpoint:        os.Getenv("AWS_ENDPOINT_URL"),
		Region:          os.Getenv("AWS_REGION"),
		Bucket:          bucketFromRoot(os.Getenv("FB_ROOT")),
	}
}

// ValidateFilesystem checks the storage root and static credential pair.
// An empty pair selects the ambient AWS credential chain.
func (c Config) ValidateFilesystem() error {
	if err := c.ValidateRoot(); err != nil {
		return err
	}
	hasAccessKey := strings.TrimSpace(c.AccessKeyID) != ""
	hasSecretKey := strings.TrimSpace(c.SecretAccessKey) != ""
	if hasAccessKey != hasSecretKey {
		return ErrMissingCredentials
	}
	return nil
}

// UsesAmbientCredentials reports whether rclone must use the AWS credential
// provider chain instead of a configured static key pair.
func (c Config) UsesAmbientCredentials() bool {
	return strings.TrimSpace(c.AccessKeyID) == "" && strings.TrimSpace(c.SecretAccessKey) == ""
}

// ValidateRoot checks that the root is the S3 service root or one bucket.
func (c Config) ValidateRoot() error {
	if c.Bucket == "." || c.Bucket == ".." ||
		strings.Contains(c.Bucket, "/") || strings.ContainsRune(c.Bucket, 0) {
		return ErrInvalidRoot
	}
	return nil
}

// ValidateUserDir checks whether generated user directories have a bucket.
func (c Config) ValidateUserDir(enabled bool) error {
	if enabled && c.Bucket == "" {
		return ErrUserDirNeedsBucket
	}
	return nil
}

// Root returns the rclone backend root. An empty root exposes S3 buckets.
func (c Config) Root() string {
	return c.Bucket
}

// SetRoot applies the server's validated storage root.
func (c *Config) SetRoot(value string) {
	c.Bucket = bucketFromRoot(value)
}

// ObjectPath maps a path in a user's view to its key below the storage root.
func ObjectPath(userScope, name string) (string, error) {
	if strings.ContainsRune(userScope, 0) || strings.ContainsRune(name, 0) {
		return "", ErrInvalidObjectPath
	}
	scopeRoot := path.Clean(path.Join("/", userScope))
	candidate := path.Clean(path.Join(scopeRoot, name))
	if scopeRoot != "/" && candidate != scopeRoot && !strings.HasPrefix(candidate, scopeRoot+"/") {
		return "", ErrInvalidObjectPath
	}
	return candidate, nil
}

func bucketFromRoot(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return ""
	}
	return value
}
