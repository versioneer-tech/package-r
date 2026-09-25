package rclonefs

import (
	"context"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"

	"github.com/versioneer-tech/package-r/objectstorage"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/users"
)

// Provider supplies the process-owned object filesystem.
type Provider interface {
	FileSystem() (afero.Fs, error)
	PublicLink(ctx context.Context, name string, expire time.Duration) (string, error)
}

type userStore struct {
	users.Store
	provider Provider
	settings *settings.Settings
}

var _ users.PublicLinker = (*userStore)(nil)

// WrapUsers replaces the local filesystem on users returned by a store.
func WrapUsers(store users.Store, provider Provider, applicationSettings *settings.Settings) users.Store {
	return &userStore{Store: store, provider: provider, settings: applicationSettings}
}

func (s *userStore) Get(baseScope string, id interface{}) (*users.User, error) {
	user, err := s.Store.Get(baseScope, id)
	if err != nil {
		return nil, err
	}
	if err := s.setFileSystem(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userStore) Gets(baseScope string) ([]*users.User, error) {
	all, err := s.Store.Gets(baseScope)
	if err != nil {
		return nil, err
	}
	for _, user := range all {
		if err := s.setFileSystem(user); err != nil {
			return nil, err
		}
	}
	return all, nil
}

func (s *userStore) PublicLink(ctx context.Context, user *users.User, name string, expire time.Duration) (string, error) {
	objectPath, err := objectstorage.ObjectPath(user.Scope, name)
	if err != nil {
		return "", err
	}
	return s.provider.PublicLink(ctx, objectPath, expire)
}

func (s *userStore) setFileSystem(user *users.User) error {
	base, err := s.provider.FileSystem()
	if err != nil {
		return err
	}
	if s.settings != nil && s.settings.CreateUserDir && !user.Perm.Admin {
		username := settings.CleanUsername(user.Username)
		userHomeBase := strings.TrimSpace(s.settings.UserHomeBasePath)
		if userHomeBase == "" {
			userHomeBase = settings.DefaultUsersHomeBasePath
		}
		home := path.Join("/", userHomeBase, username)
		if err := base.MkdirAll(home, 0o755); err != nil {
			return err
		}
		marker := path.Join(home, ".keep")
		exists, err := afero.Exists(base, marker)
		if err != nil {
			return err
		}
		if !exists {
			if err := afero.WriteFile(base, marker, []byte("created by packageR; keep this file\n"), 0o644); err != nil {
				return err
			}
		}
	}
	userScope := filepath.Join("/", user.Scope)
	rcloneAfero := newScopeFS(base, userScope)
	user.Fs = afero.NewBasePathFs(rcloneAfero, userScope)
	return nil
}
