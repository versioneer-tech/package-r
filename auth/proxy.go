package auth

import (
	"errors"
	"net/http"
	"os"

	fbErrors "github.com/versioneer-tech/package-r/v2/errors"
	"github.com/versioneer-tech/package-r/v2/settings"
	"github.com/versioneer-tech/package-r/v2/users"
)

// MethodProxyAuth is used to identify no auth.
const MethodProxyAuth settings.AuthMethod = "proxy"

// ProxyAuth is a proxy implementation of an auther.
type ProxyAuth struct {
	Header string `json:"header"`
}

// Auth authenticates the user via an HTTP header.
func (a ProxyAuth) Auth(r *http.Request, usr users.Store, _ *settings.Settings, srv *settings.Server) (*users.User, error) {
	username := r.Header.Get(a.Header)
	user, err := usr.Get(username)
	if errors.Is(err, fbErrors.ErrNotExist) {
		return nil, os.ErrPermission
	}

	return user, err
}

// LoginPage tells that proxy auth doesn't require a login page.
func (a ProxyAuth) LoginPage() bool {
	return false
}
