package files

import (
	"fmt"
	"log"
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

/*
Edge Case Coverage for Presign:

| Input path             | bucketNameOverride | Resulting bucket | Resulting key          | Outcome     |
|------------------------|--------------------|------------------|------------------------|-------------|
| "/bucket/key1.txt"     | ""                 | "bucket"         | "key1.txt"             | valid       |
| "/key1.txt"            | "bucket"           | "bucket"         | "key1.txt"             | valid       |
| "/bucket/key1.txt"     | "bucket"           | "bucket"         | "key1.txt"             | valid       |
| "/bucket/"             | "bucket"           | "bucket"         | ""                     | invalid     |
| "/"                    | "bucket"           | "bucket"         | ""                     | invalid     |
| ""                     | ""                 | -                | -                      | invalid     |
*/

func (conn *S3Connection) Presign(path, method string, cutoff int64) (string, error) {
	if conn == nil || conn.s3 == nil {
		return "", fmt.Errorf("skip presign without valid S3 connection for '%s'", path)
	}
	var bucket, key string
	bucketNameOverride := strings.TrimSpace(strings.Trim(conn.bucketName, "/"))
	trimmedPath := strings.TrimPrefix(path, "/")

	if bucketNameOverride == "" {
		if trimmedPath == "" {
			return "", fmt.Errorf("skip presign without valid path for '%s'", path)
		}
		segments := strings.Split(trimmedPath, "/")
		if len(segments) == 0 || segments[0] == "" {
			return "", fmt.Errorf("skip presign with invalid path for '%s'", path)
		}
		bucket = segments[0]
		key = strings.Join(segments[1:], "/")
	} else {
		bucket = bucketNameOverride
		if trimmedPath == "" {
			key = ""
		} else {
			segments := strings.Split(trimmedPath, "/")
			if segments[0] == bucketNameOverride {
				key = strings.Join(segments[1:], "/")
			} else {
				key = trimmedPath
			}
		}
	}

	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return "", fmt.Errorf("skip presign with empty path for '%s'", path)
	}

	if conn.bucketPrefix != "" {
		key = strings.TrimSuffix(conn.bucketPrefix, "/") + "/" + key
	}

	log.Printf("presigning (bucket: '%s', key: '%s')", bucket, key)

	var getObjectInput *s3.GetObjectInput

	if cutoff > 0 {
		cutoffTime := time.Unix(cutoff, 0)

		listObjectVersionsInput := &s3.ListObjectVersionsInput{
			Bucket: aws.String(bucket),
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
				Bucket:    aws.String(bucket),
				Key:       aws.String(key),
				VersionId: version.VersionId,
			}
			break
		}
	}

	if getObjectInput == nil {
		getObjectInput = &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		}
	}

	req, _ := conn.s3.GetObjectRequest(getObjectInput)
	req.Operation.HTTPMethod = method
	req.HTTPRequest.Method = method
	return req.Presign(7 * 24 * time.Hour)
}

func getStringOrDefault(values map[string]string, key, defaultValue string) string {
	if value, ok := values[key]; ok && value != "" {
		return value
	}
	return defaultValue
}

func Presign(path, method string, envs map[string]string) (string, error) {
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

	url, err := conn.Presign(path, method, 0)
	if err != nil {
		return "", fmt.Errorf("could not presign object %q: %w", path, err)
	}
	return url, nil
}
