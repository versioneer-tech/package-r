package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/golang-jwt/jwt/v4/request"
	"github.com/spf13/afero"

	fbErrors "github.com/versioneer-tech/package-r/v2/errors"
	"github.com/versioneer-tech/package-r/v2/k8s"
	"github.com/versioneer-tech/package-r/v2/objects"

	"github.com/versioneer-tech/package-r/v2/share"

	"github.com/versioneer-tech/package-r/v2/users"
)

const (
	DefaultTokenExpirationTime = time.Hour * 2
)

type userInfo struct {
	ID           uint              `json:"id"`
	Locale       string            `json:"locale"`
	ViewMode     users.ViewMode    `json:"viewMode"`
	SingleClick  bool              `json:"singleClick"`
	Perm         users.Permissions `json:"perm"`
	Commands     []string          `json:"commands"`
	LockPassword bool              `json:"lockPassword"`
	HideDotfiles bool              `json:"hideDotfiles"`
	DateFormat   bool              `json:"dateFormat"`
}

type authToken struct {
	User    userInfo       `json:"user"`
	Sources []share.Source `json:"sources"`
	jwt.RegisteredClaims
}

type extractor []string

func (e extractor) ExtractToken(r *http.Request) (string, error) {
	token, _ := request.HeaderExtractor{"X-Auth"}.ExtractToken(r)

	// Checks if the token isn't empty and if it contains two dots.
	// The former prevents incompatibility with URLs that previously
	// used basic auth.
	if token != "" && strings.Count(token, ".") == 2 {
		return token, nil
	}

	auth := r.URL.Query().Get("auth")
	if auth != "" && strings.Count(auth, ".") == 2 {
		return auth, nil
	}

	if r.Method == http.MethodGet {
		cookie, _ := r.Cookie("auth")
		if cookie != nil && strings.Count(cookie.Value, ".") == 2 {
			return cookie.Value, nil
		}
	}

	return "", request.ErrNoTokenInRequest
}

func withUser(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		keyFunc := func(_ *jwt.Token) (interface{}, error) {
			return d.settings.Key, nil
		}

		var tk authToken
		token, err := request.ParseFromRequest(r, &extractor{}, keyFunc, request.WithClaims(&tk))

		if err != nil || !token.Valid {
			return http.StatusUnauthorized, nil
		}

		expired := !tk.VerifyExpiresAt(time.Now().Add(time.Hour), true)
		updated := tk.IssuedAt != nil && tk.IssuedAt.Unix() < d.store.Users.LastUpdate(tk.User.ID)

		if expired || updated {
			w.Header().Add("X-Renew-Token", "true")
		}

		d.user, err = d.store.Users.Get(tk.User.ID)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		source := share.GetSource(tk.Sources, r.URL.Query().Get("sourceName"))
		if source != nil {
			session := source.Connect(*d.store.K8sCache)
			if session != nil {
				d.user.Fs = afero.NewBasePathFs(objects.NewObjectFs(source.BucketName, session), "/"+source.BucketPrefix)
			}
		}

		d.raw = tk.Sources

		return fn(w, r, d)
	}
}

func withAdmin(fn handleFunc) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Admin {
			return http.StatusForbidden, nil
		}

		return fn(w, r, d)
	})
}

func loginHandler(tokenExpireTime time.Duration) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		auther, err := d.store.Auth.Get(d.settings.AuthMethod)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		user, err := auther.Auth(r, d.store.Users, d.settings, d.server)
		switch {
		case errors.Is(err, os.ErrPermission):
			return http.StatusForbidden, nil
		case err != nil:
			return http.StatusInternalServerError, err
		}

		return printToken(w, r, d, user, tokenExpireTime)
	}
}

type signupBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var signupHandler = func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.settings.Signup {
		return http.StatusMethodNotAllowed, nil
	}

	if r.Body == nil {
		return http.StatusBadRequest, nil
	}

	info := &signupBody{}
	err := json.NewDecoder(r.Body).Decode(info)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if info.Password == "" || info.Username == "" {
		return http.StatusBadRequest, nil
	}

	user := &users.User{
		Username: info.Username,
	}

	d.settings.Defaults.Apply(user)

	pwd, err := users.HashPwd(info.Password)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	user.Password = pwd

	userHome, err := d.settings.MakeUserDir(user.Username, user.Scope, d.server.Root)
	if err != nil {
		log.Printf("create user: failed to mkdir user home dir: [%s]", userHome)
		return http.StatusInternalServerError, err
	}
	user.Scope = userHome
	log.Printf("new user: %s, home dir: [%s].", user.Username, userHome)

	err = d.store.Users.Save(user)
	if errors.Is(err, fbErrors.ErrExist) {
		return http.StatusConflict, err
	} else if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func renewHandler(tokenExpireTime time.Duration) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		w.Header().Set("X-Renew-Token", "false")
		return printToken(w, r, d, d.user, tokenExpireTime)
	})
}

//nolint:gocritic,gocyclo
func printToken(w http.ResponseWriter, _ *http.Request, d *data, user *users.User, tokenExpirationTime time.Duration) (int, error) {
	sources := map[string]share.Source{}

	for _, bucket := range strings.Split(os.Getenv("BUCKET_DEFAULT"), "|") {
		if bucket != "" {
			source := share.Source{}
			source.Name = bucket
			source.SecretName = bucket
			source.BucketName = bucket
			source.PresignSecretName = bucket
			source.PresignBucketName = bucket
			sources[source.Name] = source
		}
	}

	nsc := k8s.NewDefaultClient()
	if nsc != nil {
		ctx := context.Background()
		resp, err := nsc.ListSources(ctx)
		if err != nil {
			log.Printf("Sources couldn't be retrieved: %s", err)
		} else {
			for _, item := range resp.Items {
				if item.Spec.AllowedRoles != nil {
					continue // to be implemented with RBAC
				}
				source := share.Source{}
				source.Name = item.ObjectMeta.Name
				source.FriendlyName = item.Spec.FriendlyName
				if item.Spec.Access.BucketName != "" {
					source.BucketName = item.Spec.Access.BucketName
				} else {
					source.BucketName = item.ObjectMeta.Name // convention to use name as default
				}
				if item.Spec.Access.BucketPrefix != "" {
					source.BucketPrefix = item.Spec.Access.BucketPrefix
				}
				if item.Spec.Access.SecretName != "" {
					source.SecretName = item.Spec.Access.SecretName
				} else {
					source.SecretName = item.ObjectMeta.Name // convention to use name as default
				}
				if item.Spec.Share.BucketName != "" {
					source.PresignBucketName = item.Spec.Share.BucketName
				} else {
					source.PresignBucketName = source.BucketName // use same as for access if not provided
				}
				if item.Spec.Share.BucketPrefix != "" {
					source.PresignBucketPrefix = item.Spec.Share.BucketPrefix
				}
				if item.Spec.Share.SecretName != "" {
					source.PresignSecretName = item.Spec.Share.SecretName
				} else {
					source.PresignSecretName = source.SecretName // use same as for access if not provided
				}
				if item.Spec.SubPath != "" {
					source.SubPath = item.Spec.SubPath
				}

				sources[source.Name] = source
			}
		}
	}
	sortedSources := make([]share.Source, 0, len(sources))
	for _, source := range sources {
		sortedSources = append(sortedSources, source)
	}
	sort.Slice(sortedSources, func(i, j int) bool {
		return sortedSources[i].Name < sortedSources[j].Name
	})

	claims := &authToken{
		User: userInfo{
			ID:           user.ID,
			Locale:       user.Locale,
			ViewMode:     user.ViewMode,
			SingleClick:  user.SingleClick,
			Perm:         user.Perm,
			LockPassword: user.LockPassword,
			Commands:     user.Commands,
			HideDotfiles: user.HideDotfiles,
			DateFormat:   user.DateFormat,
		},
		Sources: sortedSources,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExpirationTime)),
			Issuer:    "packageR",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(d.settings.Key)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(signed)); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
