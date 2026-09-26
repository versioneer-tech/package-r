package http

import (
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/tomasen/realip"

	"github.com/versioneer-tech/package-r/rules"
	"github.com/versioneer-tech/package-r/runner"
	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/storage"
	"github.com/versioneer-tech/package-r/users"
)

type handleFunc func(w http.ResponseWriter, r *http.Request, d *data) (int, error)

type data struct {
	*runner.Runner
	settings *settings.Settings
	server   *settings.Server
	store    *storage.Storage
	user     *users.User
	raw      interface{}

	skipUserDirBaseRules bool
}

// Check implements rules.Checker.
func (d *data) Check(path string) bool {
	path = cleanAccessPath(path)
	if d.user.HideDotfiles && rules.MatchHidden(path) {
		return false
	}

	allow := true
	for _, rule := range d.settings.Rules {
		if rule.Matches(path) {
			allow = rule.Allow
		}
	}

	for _, rule := range d.user.Rules {
		if d.skipUserDirBaseRules && d.settings.IsGeneratedUserDirBaseRule(d.user.Username, rule) {
			continue
		}
		if rule.Matches(path) {
			allow = rule.Allow
		}
	}

	if !d.skipUserDirBaseRules && !d.checkUserDirPath(path) {
		return false
	}

	return allow
}

// CheckWrite applies path rules and protects the user-directory base from
// recursive mutations. The base remains readable so users can navigate to
// their own directory.
func (d *data) CheckWrite(requestPath string) bool {
	requestPath = cleanAccessPath(requestPath)
	if requestPath == "/" || !d.Check(requestPath) {
		return false
	}
	if !d.settings.CreateUserDir {
		return true
	}

	return !pathAtOrAbove(requestPath, d.userDirBasePath())
}

func (d *data) checkUserDirPath(requestPath string) bool {
	if !d.settings.CreateUserDir {
		return true
	}

	basePath := d.userDirBasePath()
	if requestPath == basePath || !pathAtOrBelow(requestPath, basePath) {
		return true
	}

	username := settings.CleanUsername(d.user.Username)
	if username == "" || username == "-" || username == "." {
		return false
	}

	return pathAtOrBelow(requestPath, path.Join(basePath, username))
}

func (d *data) userDirBasePath() string {
	basePath := strings.TrimSpace(d.settings.UserHomeBasePath)
	if basePath == "" {
		basePath = settings.DefaultUsersHomeBasePath
	}
	return cleanAccessPath(basePath)
}

func cleanAccessPath(requestPath string) string {
	return path.Clean("/" + strings.TrimPrefix(requestPath, "/"))
}

func pathAtOrBelow(requestPath, basePath string) bool {
	return requestPath == basePath ||
		(basePath == "/" && strings.HasPrefix(requestPath, "/")) ||
		strings.HasPrefix(requestPath, basePath+"/")
}

func pathAtOrAbove(requestPath, basePath string) bool {
	return requestPath == "/" || requestPath == basePath || strings.HasPrefix(basePath, requestPath+"/")
}

func handle(fn handleFunc, prefix string, store *storage.Storage, server *settings.Server) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range globalHeaders {
			w.Header().Set(k, v)
		}

		settings, err := store.Settings.Get()
		if err != nil {
			log.Fatalf("ERROR: couldn't get settings: %v\n", err)
			return
		}

		status, err := fn(w, r, &data{
			Runner:   &runner.Runner{Enabled: server.EnableExec, Settings: settings},
			store:    store,
			settings: settings,
			server:   server,
		})

		if status >= 400 || err != nil {
			clientIP := realip.FromRequest(r)
			log.Printf("%s: %v %s %v", r.URL.Path, status, clientIP, err)
		}

		if status != 0 && status != http.StatusTemporaryRedirect {
			txt := http.StatusText(status)
			http.Error(w, strconv.Itoa(status)+" "+txt, status)
			return
		}
	})

	return stripPrefix(prefix, handler)
}
