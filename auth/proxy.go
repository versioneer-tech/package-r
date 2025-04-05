package auth

import (
	"context"
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

const MethodProxyAuth settings.AuthMethod = "proxy"

type ProxyAuth struct {
	Header string `json:"header"`
	Mapper string `json:"mapper"`
}

func (a ProxyAuth) extractUsername(claims map[string]interface{}) (string, bool) {
	if strings.HasPrefix(a.Mapper, ".") {
		key := a.Mapper[1:]
		if strVal, ok := claims[key].(string); ok {
			return strVal, true
		}
		return "", false
	}

	if a.Mapper == "azp-groups" {
		azp, ok := claims["azp"].(string)
		if !ok {
			return "", false
		}
		groups, ok := claims["groups"].([]interface{})
		if !ok {
			return "", false
		}
		for _, group := range groups {
			if str, ok := group.(string); ok && str == azp {
				if adminVal, ok := claims["admin"]; ok {
					switch v := adminVal.(type) {
					case string:
						if v == "true" {
							return "admin", true
						}
					case bool:
						if v {
							return "admin", true
						}
					}
				}
				return "user", true
			}
		}
		return "guest", true
	}
	return "", false
}

func (a ProxyAuth) Auth(r *http.Request, usr users.Store, _ *settings.Settings, srv *settings.Server) (*users.User, error) {
	header := r.Header.Get(a.Header)
	if header == "" {
		log.Printf("Missing header %s", a.Header)
		return nil, os.ErrPermission
	}
	var username string
	if a.Mapper != "" {
		if strings.HasPrefix(a.Mapper, "=") {
			username = a.Mapper[1:]
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
				context.WithValue(r.Context(), "claims", claims)
				username, ok = a.extractUsername(claims)
				if !ok || username == "" {
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
				context.WithValue(r.Context(), "claims", claims)
				var ok bool
				username, ok = a.extractUsername(claims)
				if !ok || username == "" {
					log.Printf("Missing base64 claim %s in %s", a.Mapper[1:], header)
					return nil, os.ErrPermission
				}
			}
		}
	}

	user, err := usr.Get(srv.Root, username)
	if errors.Is(err, fbErrors.ErrNotExist) {
		log.Printf("User %s not found", username)
		return nil, os.ErrPermission
	}

	return user, err
}

func (a ProxyAuth) LoginPage() bool {
	return false
}
