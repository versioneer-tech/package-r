package bolt

import (
	"github.com/asdine/storm/v3"

	"github.com/versioneer-tech/package-r/auth"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/share"
	"github.com/versioneer-tech/package-r/storage"
	"github.com/versioneer-tech/package-r/users"
)

// NewStorage creates a storage.Storage based on Bolt DB.
func NewStorage(db *storm.DB) *storage.Storage {
	userStore := users.NewStorage(usersBackend{db: db})
	shareStore := share.NewStorage(shareBackend{db: db})
	settingsStore := settings.NewStorage(settingsBackend{db: db})
	authStore := auth.NewStorage(authBackend{db: db}, userStore)

	return &storage.Storage{
		Auth:     authStore,
		Users:    userStore,
		Share:    shareStore,
		Settings: settingsStore,
	}
}
