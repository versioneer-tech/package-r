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

func (s *Source) Connect() (string, *awsSession.Session) {
	if s.Name == "" {
		return "Source information missing", nil
	}

	values := map[string]string{}
	if s.SecretName != "" {
		nsc := k8s.NewDefaultClient()
		ctx := context.Background()
		resp, err := nsc.GetSecret(ctx, s.SecretName)
		if err == nil && resp != nil {
			log.Printf("Secret <- %+v", resp)
			for k, v := range resp.Data {
				values[k] = string(v)
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

	bucket := GetStringOrDefault(values, "BUCKET_NAME", s.Name)

	if errSession != nil {
		log.Print("Could not create session:", errSession)
		return "", nil
	}

	return bucket, session
}

func (s *Source) Presign(keys []string) (presignedUrls []string, status int, err error) {
	presignedURLs := []string{}
	bucket, session := s.Connect()
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

func GetStringOrDefault(values map[string]string, key, defaultValue string) string {
	if value, ok := values[key]; ok {
		return value
	}
	return defaultValue
}
