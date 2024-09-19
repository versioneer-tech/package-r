package share

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	v1 "k8s.io/api/core/v1"

	"github.com/versioneer-tech/package-r/v2/k8s"
)

type Source struct {
	Name              string `json:"name"`
	FriendlyName      string `json:"friendlyName,omitempty"`
	SecretName        string `json:"secretName,omitempty"`
	PresignSecretName string `json:"presignSecretName,omitempty"`
	SubPath           string `json:"subPath,omitempty"`
}

func (s *Source) Connect(k8sCache k8s.Cache) (bucket, prefix string, session *awsSession.Session) {
	return connect(s.Name, s.SecretName, k8sCache)
}

func connect(sourceName, secretName string, k8sCache k8s.Cache) (bucket, prefix string, session *awsSession.Session) {
	if sourceName == "" {
		log.Print("Source information missing")
		return "", "", nil
	}

	values := map[string]string{}

	if secretName != "" {
		resp, err := k8sCache.GetSecret(secretName, func(s string) (*v1.Secret, error) {
			nsc := k8s.NewDefaultClient()
			ctx := context.Background()
			log.Printf("GetSecret for %s -> %s", sourceName, secretName)
			return nsc.GetSecret(ctx, secretName)
		})
		if err == nil && resp != nil {
			for k, v := range resp.Data {
				values[k] = string(v)
			}

		} else {
			log.Printf("Could not get secret: %s", secretName)
		}
	}

	session, errSession := awsSession.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			GetStringOrDefault(values, "AWS_ACCESS_KEY_ID", os.Getenv("AWS_ACCESS_KEY_ID")),
			GetStringOrDefault(values, "AWS_SECRET_ACCESS_KEY", os.Getenv("AWS_SECRET_ACCESS_KEY")),
			""),
		Endpoint:         aws.String(GetStringOrDefault(values, "AWS_ENDPOINT_URL", os.Getenv("AWS_ENDPOINT_URL"))),
		Region:           aws.String(GetStringOrDefault(values, "AWS_REGION", os.Getenv("AWS_REGION"))),
		S3ForcePathStyle: aws.Bool(true),
		//LogLevel:         aws.LogLevel(aws.LogDebugWithHTTPBody),
	})

	bucket = GetStringOrDefault(values, "BUCKET_NAME", sourceName)
	prefix = GetStringOrDefault(values, "BUCKET_PREFIX", "")

	if errSession != nil {
		log.Printf("Could not create session: %s", errSession)
		return "", "", nil
	}

	return bucket, prefix, session
}

func Presign(source *Source, k8sCache k8s.Cache, paths ...string) (presignedUrls []string, status int, err error) {
	presignedURLs := []string{}
	_, prefix, _ := connect(source.Name, source.SecretName, k8sCache)
	bucket, _, session := connect(source.Name, source.PresignSecretName, k8sCache)
	if session != nil {
		s3Client := s3.New(session)
		for _, path := range paths {
			path = strings.TrimPrefix(path, "/")
			path = strings.TrimPrefix(path, prefix)
			if source.SubPath != "" {
				path = source.SubPath + "/" + path
			}
			getObjectInput := s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(path),
			}

			req, _ := s3Client.GetObjectRequest(&getObjectInput)

			presignedURL, err := req.Presign(7 * 24 * time.Hour) // 7d max on AWS
			if err != nil {
				log.Printf("Could not presign %v: %v", getObjectInput, err)
				return presignedURLs, http.StatusInternalServerError, err
			}

			presignedURLs = append(presignedURLs, presignedURL)
		}
	}

	return presignedURLs, 0, nil
}

func GetStringOrDefault(values map[string]string, key, defaultValue string) string {
	if value, ok := values[key]; ok {
		return value
	}
	return defaultValue
}

func GetSource(sources []Source, sourceName string) *Source {
	if sources != nil && sourceName != "" {
		for _, source := range sources {
			if source.Name == sourceName {
				return &source
			}
		}
	}
	return nil
}
