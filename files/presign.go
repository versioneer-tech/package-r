package files

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3Connection struct {
	s3           *s3.S3
	bucketName   string
	bucketPrefix string
}

func (conn *S3Connection) Presign(key string, cutoff int64) (string, error) {
	if conn == nil || conn.s3 == nil {
		return "", fmt.Errorf("presign without valid S3 connection (bucket: %s, key: %s)", conn.bucketName, key)
	}
	if key == "" {
		return "", fmt.Errorf("presign with empty key (bucket: %s)", conn.bucketName)
	}

	key = strings.TrimPrefix(key, "/")
	key = strings.TrimPrefix(key, conn.bucketName+"/")

	if conn.bucketPrefix != "" {
		if !strings.HasSuffix(conn.bucketPrefix, "/") {
			key = conn.bucketPrefix + "/" + key
		} else {
			key = conn.bucketPrefix + key
		}
	}

	var getObjectInput *s3.GetObjectInput

	if cutoff > 0 {
		cutoffTime := time.Unix(cutoff, 0)

		listObjectVersionsInput := &s3.ListObjectVersionsInput{
			Bucket: aws.String(conn.bucketName),
			Prefix: aws.String(key),
		}

		listOutput, err := conn.s3.ListObjectVersions(listObjectVersionsInput)
		if err != nil {
			return "", fmt.Errorf("failed to list object versions: %w", err)
		}

		for _, version := range listOutput.Versions {
			if version.LastModified != nil && version.LastModified.After(cutoffTime) {
				continue
			}
			getObjectInput = &s3.GetObjectInput{
				Bucket:    aws.String(conn.bucketName),
				Key:       aws.String(key),
				VersionId: version.VersionId,
			}
			break
		}
	}

	if getObjectInput == nil {
		getObjectInput = &s3.GetObjectInput{
			Bucket: aws.String(conn.bucketName),
			Key:    aws.String(key),
		}
	}

	req, _ := conn.s3.GetObjectRequest(getObjectInput)
	return req.Presign(7 * 24 * time.Hour)
}

func getStringOrDefault(values map[string]string, key, defaultValue string) string {
	if value, ok := values[key]; ok && value != "" {
		return value
	}
	return defaultValue
}

func Presign(path string, envs map[string]string) (string, error) {
	sess, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			getStringOrDefault(envs, "AWS_ACCESS_KEY_ID", os.Getenv("AWS_ACCESS_KEY_ID")),
			getStringOrDefault(envs, "AWS_SECRET_ACCESS_KEY", os.Getenv("AWS_SECRET_ACCESS_KEY")),
			"",
		),
		Endpoint:         aws.String(getStringOrDefault(envs, "AWS_ENDPOINT_URL", os.Getenv("AWS_ENDPOINT_URL"))),
		Region:           aws.String(getStringOrDefault(envs, "AWS_REGION", os.Getenv("AWS_REGION"))),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("could not create S3 session: %w", err)
	}

	conn := S3Connection{
		s3:           s3.New(sess),
		bucketName:   getStringOrDefault(envs, "BUCKET_NAME", ""),
		bucketPrefix: getStringOrDefault(envs, "BUCKET_PREFIX", ""),
	}

	url, err := conn.Presign(path, 0)
	if err != nil {
		return "", fmt.Errorf("could not presign object %q: %w", path, err)
	}
	return url, nil
}
