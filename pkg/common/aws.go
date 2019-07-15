package common

import (
	"os"
	"path"
	"path/filepath"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type S3Client interface {
	Download(file *os.File, bucket, itemKey, region, accessKeyID, secretAccessKey, sessionToken string) error
}

type AbstractS3Client struct {
	S3Client
}

func (asc AbstractS3Client) Download(file *os.File, bucket, itemKey, region, accessKeyID, secretAccessKey, sessionToken string) error {
	itemKey = filepath.Clean(itemKey)

	pathInsideBucket, item := path.Split(itemKey)
	bucketAndPath := path.Join(bucket, pathInsideBucket) + "/"

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"), // default region (some valid region must be mentioned)
		Credentials: credentials.NewStaticCredentials(accessKeyID, secretAccessKey, sessionToken),
	})
	if err != nil {
		return errors.Wrap(err, "Failed to create AWS session")
	}

	// get the bucket's region in case it wasn't given
	if region == "" {
		region, err = s3manager.GetBucketRegion(aws.BackgroundContext(), sess, bucketAndPath, "")
		if err != nil {
			return errors.Wrap(err, "Failed to get bucket region")
		}
	}
	sess.Config.Region = aws.String(region)

	downloader := s3manager.NewDownloader(sess)
	_, err = downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: aws.String(bucketAndPath),
			Key:    aws.String(item),
		})
	if err != nil {
		return errors.Wrap(err, "Failed to download file from s3")
	}

	return nil
}
