package storage

import (
	"github.com/versioneer-tech/package-r/v2/auth"
	"github.com/versioneer-tech/package-r/v2/k8s"
	"github.com/versioneer-tech/package-r/v2/settings"
	"github.com/versioneer-tech/package-r/v2/share"
	"github.com/versioneer-tech/package-r/v2/users"
)

// Storage is a storage powered by a Backend which makes the necessary
// verifications when fetching and saving data to ensure consistency.
type Storage struct {
	Users    users.Store
	Share    *share.Storage
	Auth     *auth.Storage
	Settings *settings.Storage
	K8sCache *k8s.Cache
}
