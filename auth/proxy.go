package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
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

func extractClaimValue(claims map[string]interface{}, key string) (string, bool) {
	if strVal, ok := claims[key].(string); ok {
		return strVal, true
	}
	return "", false
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

// mapping strategy:
//
// case 1: "" #empty (default)
//
//	returns the header value
//
// case 2: ".<claim>" #i.e. starting with .
//
//	expects header value to be either a JSON or JWT and the claim
//	to exist returns the extracted claim value
//
// case 3: "<static>" #ianything else
//
//	returns the static value
func (a ProxyAuth) Extract(r *http.Request) (string, bool) {
	header := r.Header.Get(a.Header)
	if header == "" {
		return "", false
	}
	if a.Mapper == "" {
		return header, true
	}
	if a.Mapper[0] != '.' {
		return a.Mapper, true
	}
	claims, ok := extractClaims(header)
	if !ok {
		return "", false
	}
	return extractClaimValue(claims, a.Mapper[2:])
}

// Auth authenticates the user via an HTTP header.
func (a ProxyAuth) Auth(r *http.Request, usr users.Store, setting *settings.Settings, srv *settings.Server) (*users.User, error) {
	if a.Header == "" {
		log.Println("Missing auth.header config")
		return nil, fbErrors.ErrInvalidAuthMethod
	}
	username, ok := a.Extract(r)
	if !ok {
		log.Printf("No value can be inferred from header %s with mapper %s", a.Header, a.Mapper)
		return nil, fbErrors.ErrNotExist
	}
	user, err := usr.Get(srv.Root, username)
	if errors.Is(err, fbErrors.ErrNotExist) {
		if setting.Signup {
			return a.createUser(usr, setting, srv, username)
		}
		log.Printf("User %s not found", username)
	}
	return user, err
}

func (a ProxyAuth) createUser(usr users.Store, setting *settings.Settings, srv *settings.Server, username string) (*users.User, error) {
	const passwordSize = 32
	randomPasswordBytes := make([]byte, passwordSize)
	_, err := rand.Read(randomPasswordBytes)
	if err != nil {
		return nil, err
	}

	var hashedRandomPassword string
	hashedRandomPassword, err = users.HashPwd(string(randomPasswordBytes))
	if err != nil {
		return nil, err
	}

	user := &users.User{
		Username:     username,
		Password:     hashedRandomPassword,
		LockPassword: true,
	}
	setting.Defaults.Apply(user)

	var userHome string
	userHome, err = setting.MakeUserDir(user.Username, user.Scope, srv.Root)
	if err != nil {
		return nil, err
	}
	user.Scope = userHome

	err = usr.Save(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// LoginPage tells that proxy auth doesn't require a login page.
func (a ProxyAuth) LoginPage() bool {
	return false
}
