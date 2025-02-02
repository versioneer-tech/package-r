package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v4"

	fbErrors "github.com/versioneer-tech/package-r/errors"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/users"
)

// MethodProxyAuth is used to identify no auth.
const MethodProxyAuth settings.AuthMethod = "proxy"

// ProxyAuth is a proxy implementation of an auther.
type ProxyAuth struct {
	Header string `json:"header"`
	Mapper string `json:"mapper"`
}

// Auth authenticates the user via an HTTP header.
func (a ProxyAuth) Auth(r *http.Request, usr users.Store, _ *settings.Settings, srv *settings.Server) (*users.User, error) {
	header := r.Header.Get(a.Header)
	if a.Mapper != "" {
		if !strings.HasPrefix(a.Mapper, ".") {
			header = a.Mapper
		} else {
			if strings.Count(header, ".") == 2 {
				token, _, err := jwt.NewParser().ParseUnverified(header, jwt.MapClaims{})
				if err != nil {
					log.Printf("Invalid JWT token in %s", header)
					return nil, os.ErrPermission
				}
				claims, ok := token.Claims.(jwt.MapClaims)
				if !ok {
					log.Printf("Invalid JWT claims in %s", header)
					return nil, os.ErrPermission
				}
				header, ok = claims[a.Mapper[1:]].(string)
				if !ok || header == "" {
					log.Printf("Missing JWT claim %s in %s", a.Mapper[1:], header)
					return nil, os.ErrPermission
				}
			} else {
				token, err := base64.StdEncoding.DecodeString(header)
				if err != nil {
					log.Printf("Invalid base64 token in %s", header)
					return nil, os.ErrPermission
				}
				var claims map[string]interface{}
				err = json.Unmarshal(token, &claims)
				if err != nil {
					log.Printf("Invalid base64 claims in %s", header)
					return nil, os.ErrPermission
				}
				var ok bool
				header, ok = claims[a.Mapper[1:]].(string)
				if !ok || header == "" {
					log.Printf("Missing base64 claim %s in %s", a.Mapper[1:], header)
					return nil, os.ErrPermission
				}
			}
		}
	}

	user, err := usr.Get(srv.Root, header)
	if errors.Is(err, fbErrors.ErrNotExist) {
		log.Printf("User %s not found", header)
		return nil, os.ErrPermission
	}

	return user, err
}

// LoginPage tells that proxy auth doesn't require a login page.
func (a ProxyAuth) LoginPage() bool {
	return false
}
