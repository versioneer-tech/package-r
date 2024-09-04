package http

import (
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/spf13/afero"
	"github.com/tomasen/realip"

	"github.com/versioneer-tech/package-r/v2/rules"
	"github.com/versioneer-tech/package-r/v2/runner"
	"github.com/versioneer-tech/package-r/v2/s3fs"
	"github.com/versioneer-tech/package-r/v2/settings"
	"github.com/versioneer-tech/package-r/v2/storage"
	"github.com/versioneer-tech/package-r/v2/users"
)

type handleFunc func(w http.ResponseWriter, r *http.Request, d *data) (int, error)

type data struct {
	*runner.Runner
	settings *settings.Settings
	server   *settings.Server
	store    *storage.Storage
	user     *users.User
	raw      interface{}
}

// Check implements rules.Checker.
func (d *data) Check(path string) bool {
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
		if rule.Matches(path) {
			allow = rule.Allow
		}
	}

	return allow
}

func (d *data) InitFs(URL string) afero.Fs {
	if len(URL) == 0 {
		return nil
	}

	parsedURL, err := neturl.Parse(URL)
	if err != nil {
		return nil
	}

	contextParam := parsedURL.Query().Get("context")
	if contextParam == "" {
		contextParam = "default"
	}

	context := d.settings.Contexts[contextParam]

	if context == nil {
		context = map[string]string{}
		context["BUCKET_NAME"] = contextParam
	}

	session, errSession := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			GetStringOrDefault(context, "AWS_ACCESS_KEY_ID", os.Getenv("AWS_ACCESS_KEY_ID")),
			GetStringOrDefault(context, "AWS_SECRET_ACCESS_KEY", os.Getenv("AWS_SECRET_ACCESS_KEY")),
			""),
		Endpoint:         aws.String(GetStringOrDefault(context, "AWS_ENDPOINT_URL", os.Getenv("AWS_ENDPOINT_URL"))),
		Region:           aws.String(GetStringOrDefault(context, "AWS_REGION", os.Getenv("AWS_REGION"))),
		S3ForcePathStyle: aws.Bool(true),
	})

	bucket := GetStringOrDefault(context, "BUCKET_NAME", contextParam)

	if errSession != nil {
		log.Print("Could not create session:", errSession)
		return nil
	}

	return s3fs.NewFs(bucket, session)
}

func GetStringOrDefault(m map[string]string, key, defaultValue string) string {
	if val, ok := m[key]; ok {
		return val
	}
	return defaultValue
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

		if status != 0 {
			txt := http.StatusText(status)
			http.Error(w, strconv.Itoa(status)+" "+txt, status)
			return
		}
	})

	return stripPrefix(prefix, handler)
}
