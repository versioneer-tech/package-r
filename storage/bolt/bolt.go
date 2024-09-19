package bolt

import (
	"github.com/asdine/storm/v3"

	"github.com/versioneer-tech/package-r/v2/auth"
	"github.com/versioneer-tech/package-r/v2/k8s"
	"github.com/versioneer-tech/package-r/v2/settings"
	"github.com/versioneer-tech/package-r/v2/share"
	"github.com/versioneer-tech/package-r/v2/storage"
	"github.com/versioneer-tech/package-r/v2/users"
)

// NewStorage creates a storage.Storage based on Bolt DB.
func NewStorage(db *storm.DB) (*storage.Storage, error) {
	userStore := users.NewStorage(usersBackend{db: db})
	shareStore := share.NewStorage(shareBackend{db: db})
	settingsStore := settings.NewStorage(settingsBackend{db: db})
	authStore := auth.NewStorage(authBackend{db: db}, userStore)
	k8sCache := k8s.NewCache() // no need to persist at the moment

	err := save(db, "version", 2)
	if err != nil {
		return nil, err
	}

	return &storage.Storage{
		Auth:     authStore,
		Users:    userStore,
		Share:    shareStore,
		Settings: settingsStore,
		K8sCache: k8sCache,
	}, nil
}
