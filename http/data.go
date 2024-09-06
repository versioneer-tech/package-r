package http

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/tomasen/realip"

	"github.com/versioneer-tech/package-r/v2/rules"
	"github.com/versioneer-tech/package-r/v2/runner"
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

func (d *data) Connect(sourceName string) (string, *awsSession.Session) {
	if sourceName == "" {
		return "", nil
	}

	source := d.settings.Sources[sourceName]

	if source == nil {
		source = map[string]string{}
		source["BUCKET_NAME"] = sourceName
	}

	session, errSession := awsSession.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			GetStringOrDefault(source, "AWS_ACCESS_KEY_ID", os.Getenv("AWS_ACCESS_KEY_ID")),
			GetStringOrDefault(source, "AWS_SECRET_ACCESS_KEY", os.Getenv("AWS_SECRET_ACCESS_KEY")),
			""),
		Endpoint:         aws.String(GetStringOrDefault(source, "AWS_ENDPOINT_URL", os.Getenv("AWS_ENDPOINT_URL"))),
		Region:           aws.String(GetStringOrDefault(source, "AWS_REGION", os.Getenv("AWS_REGION"))),
		S3ForcePathStyle: aws.Bool(true),
	})

	bucket := GetStringOrDefault(source, "BUCKET_NAME", sourceName)

	if errSession != nil {
		log.Print("Could not create session:", errSession)
		return "", nil
	}

	return bucket, session
}

func (d *data) Presign(sourceName string, keys []string) (presignedUrls []string, status int, err error) {
	presignedURLs := []string{}
	bucket, session := d.Connect(sourceName)
	if session != nil {
		s3Client := s3.New(session)
		for _, key := range keys {
			getObjectInput := s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			}

			req, _ := s3Client.GetObjectRequest(&getObjectInput)

			presignedURL, err := req.Presign(7 * 24 * time.Hour) // 7d
			if err != nil {
				log.Printf("Could not presign %v: %v", getObjectInput, err)
				return presignedURLs, http.StatusInternalServerError, err
			}

			presignedURLs = append(presignedURLs, presignedURL)
		}
	}

	return presignedURLs, 0, nil
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
