package common

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func DownloadFileFromAWSS3(file *os.File, bucket, itemKey, region, accessKeyID, secretAccessKey, sessionToken string) error {
	itemKey = filepath.Clean(itemKey)
	splitPath := strings.Split(itemKey, "/")

	item := splitPath[len(splitPath)-1]

	bucketWithPathSlice := append([]string{bucket}, splitPath[:len(splitPath)-1]...)
	bucket = strings.Join(bucketWithPathSlice, "/") + "/"

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"), // default region (some valid region must be mentioned)
		Credentials: credentials.NewStaticCredentials(accessKeyID, secretAccessKey, sessionToken),
	})
	if err != nil {
		return errors.Wrap(err, "Failed to create AWS session")
	}

	// get the bucket's region in case it wasn't given
	if region == "" {
		region, err = s3manager.GetBucketRegion(aws.BackgroundContext(), sess, bucket, "")
		if err != nil {
			return errors.Wrap(err, "Failed to get bucket region")
		}
	}
	sess.Config.Region = aws.String(region)

	downloader := s3manager.NewDownloader(sess)
	_, err = downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(item),
		})
	if err != nil {
		return err
	}

	return nil
}
