package share

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/versioneer-tech/package-r/v2/k8s"
)

type Source struct {
	Name         string `json:"name"`
	FriendlyName string `json:"friendlyName,omitempty"`
	SecretName   string `json:"secretName,omitempty"`
}

func (s *Source) Connect(secretName string) (bucket, prefix string, session *awsSession.Session) {
	if s.Name == "" {
		log.Print("Source information missing")
		return "", "", nil
	}

	values := map[string]string{}

	if s.SecretName != "" || secretName != "" {
		nsc := k8s.NewDefaultClient()
		ctx := context.Background()

		if s.SecretName != "" {
			resp, err := nsc.GetSecret(ctx, s.SecretName)
			if err == nil && resp != nil {
				log.Printf("Secret <- %+v", resp)
				for k, v := range resp.Data {
					values[k] = string(v)
				}
			}
		}

		if secretName != "" {
			resp, err := nsc.GetSecret(ctx, secretName)
			if err == nil && resp != nil {
				log.Printf("Secret <- %+v", resp)
				for k, v := range resp.Data {
					values[k] = string(v)
				}
			}
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
	})

	bucket = GetStringOrDefault(values, "BUCKET_NAME", s.Name)
	prefix = GetStringOrDefault(values, "BUCKET_PREFIX", "")

	if errSession != nil {
		log.Printf("Could not create session: %s", errSession)
		return "", "", nil
	}

	return bucket, prefix, session
}

func (s *Source) Presign(secretName string, keys []string) (presignedUrls []string, status int, err error) {
	presignedURLs := []string{}
	bucket, prefix, session := s.Connect(secretName)
	if session != nil {
		s3Client := s3.New(session)
		for _, key := range keys {
			getObjectInput := s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(prefix + key),
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

func GetStringOrDefault(values map[string]string, key, defaultValue string) string {
	if value, ok := values[key]; ok {
		return value
	}
	return defaultValue
}
