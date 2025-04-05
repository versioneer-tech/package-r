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

const MethodProxyAuth settings.AuthMethod = "proxy"

type ProxyAuth struct {
	Header string `json:"header"`
	Mapper string `json:"mapper"`
}

func extractClaimValue(claims map[string]interface{}, key string) (string, bool) {
	if strVal, ok := claims[key].(string); ok {
		return strVal, true
	}
	return "", false
}

func extractClaimValues(claims map[string]interface{}, key string) ([]interface{}, bool) {
	if vals, ok := claims[key].([]interface{}); ok {
		return vals, true
	}
	return nil, false
}

func extractClaims(header string) (map[string]interface{}, bool) {
	if strings.Count(header, ".") == 2 {
		token, _, err := jwt.NewParser().ParseUnverified(header, jwt.MapClaims{})
		if err != nil {
			log.Printf("Invalid JWT token in %s", header)
			return nil, false
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			log.Printf("Invalid JWT claims in %s", header)
			return nil, false
		}
		return claims, true
	}

	token, err := base64.StdEncoding.DecodeString(header)
	if err != nil {
		log.Printf("Invalid base64 token in %s", header)
		return nil, false
	}
	var claims map[string]interface{}
	err = json.Unmarshal(token, &claims)
	if err != nil {
		log.Printf("Invalid base64 claims in %s", header)
		return nil, false
	}
	return claims, true
}

// mapper logic:
// "." full header
// "user" static value
// ".role" dynamic value from header
// "^team1" check groups with static value
// "^.azp" check groups with inferred value from header
//
//nolint:goconst,gocritic
func (a ProxyAuth) Extract(r *http.Request) (string, bool, bool) {
	header := r.Header.Get(a.Header)
	if header == "" || a.Mapper == "" {
		return "", false, false
	}
	if a.Mapper == "." {
		return header, header == "admin", true
	}
	if a.Mapper[0] != '.' && a.Mapper[0] != '^' {
		return a.Mapper, a.Mapper == "admin", false
	}
	claims, ok := extractClaims(header)
	if !ok {
		return "", false, false
	}
	admin, ok := extractClaimValue(claims, "admin")
	if !ok {
		admin = "false"
	}
	if a.Mapper[0] != '^' {
		str, ok2 := extractClaimValue(claims, a.Mapper[2:])
		return str, admin == "true", ok2
	}
	expectedStr := a.Mapper[1:]
	if a.Mapper[1] == '.' {
		expectedStr, ok = extractClaimValue(claims, a.Mapper[2:])
		if !ok {
			return "", false, false
		}
	}
	groups, ok := extractClaimValues(claims, "groups")
	if !ok {
		return "", false, false
	}
	for _, group := range groups {
		if str, ok := group.(string); ok && str == expectedStr {
			return str, admin == "true", true
		}
	}
	return "", false, false
}

func (a ProxyAuth) Auth(r *http.Request, usr users.Store, _ *settings.Settings, srv *settings.Server) (*users.User, error) {
	value, isAdmin, ok := a.Extract(r)
	if !ok {
		log.Printf("No value can be inferred from %s with %s", a.Header, a.Mapper)
		return nil, os.ErrPermission
	}
	user, err := usr.Get(srv.Root, value)
	if errors.Is(err, fbErrors.ErrNotExist) {
		if isAdmin {
			user, err = usr.Get(srv.Root, "admin")
		} else {
			user, err = usr.Get(srv.Root, "user")
		}
		if errors.Is(err, fbErrors.ErrNotExist) {
			log.Printf("User %s not found", value)
			return nil, os.ErrPermission
		}
	}
	return user, err
}

func (a ProxyAuth) LoginPage() bool {
	return false
}
