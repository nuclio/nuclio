/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"os"
	"path"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/nuclio/errors"
	"github.com/stretchr/testify/mock"
)

type S3Client interface {
	Download(file *os.File, bucket, itemKey, region, accessKeyID, secretAccessKey, sessionToken string) error
	DownloadWithinEC2Instance(file *os.File, bucket, itemKey string) error
}

type AbstractS3Client struct {
	S3Client
}

func (asc *AbstractS3Client) Download(file *os.File,
	bucket string,
	itemKey string,
	region string,
	accessKeyID string,
	secretAccessKey string,
	sessionToken string) error {
	bucketAndPath, item := asc.resolveBucketPathAndItem(bucket, itemKey)
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

	return asc.download(file, sess, bucketAndPath, item)
}

func (asc *AbstractS3Client) DownloadWithinEC2Instance(file *os.File, bucket, itemKey string) error {
	bucketAndPath, item := asc.resolveBucketPathAndItem(bucket, itemKey)
	sess, err := session.NewSession()
	if err != nil {
		return errors.Wrap(err, "Failed to create session")
	}
	return asc.download(file, sess, bucketAndPath, item)

}

func (asc *AbstractS3Client) resolveBucketPathAndItem(bucket, itemKey string) (string, string) {
	pathInsideBucket, item := path.Split(filepath.Clean(itemKey))
	bucketAndPath := path.Join(bucket, pathInsideBucket) + "/"
	return bucketAndPath, item
}

func (asc *AbstractS3Client) download(file *os.File, sess *session.Session, bucketAndPath, item string) error {
	downloader := s3manager.NewDownloader(sess)
	if _, err := downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: aws.String(bucketAndPath),
			Key:    aws.String(item),
		}); err != nil {
		return errors.Wrap(err, "Failed to download file from s3")
	}
	return nil
}

type MockS3Client struct {
	mock.Mock
	FilePath string
}

func (msc *MockS3Client) Download(file *os.File, bucket, itemKey, region, accessKeyID, secretAccessKey, sessionToken string) error {
	functionArchiveFileBytes, _ := os.ReadFile(msc.FilePath)

	_ = os.WriteFile(file.Name(), functionArchiveFileBytes, os.FileMode(os.O_RDWR))

	args := msc.Called(file, bucket, itemKey, region, accessKeyID, secretAccessKey, sessionToken)
	return args.Error(0)
}

func (msc *MockS3Client) DownloadWithinEC2Instance(file *os.File, bucket, itemKey string) error {
	functionArchiveFileBytes, _ := os.ReadFile(msc.FilePath)

	_ = os.WriteFile(file.Name(), functionArchiveFileBytes, os.FileMode(os.O_RDWR))

	args := msc.Called(file, bucket, itemKey)
	return args.Error(0)
}
