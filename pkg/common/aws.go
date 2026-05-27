/*
Copyright 2023 The Nuclio Authors.

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
	"context"
	"os"
	"path"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/nuclio/errors"
	"github.com/stretchr/testify/mock"
)

const DefaultRegion = "us-east-1"

type S3Client interface {
	Download(file *os.File, bucket, itemKey, region, accessKeyID, secretAccessKey, sessionToken, endpoint string) error
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
	sessionToken string,
	endpoint string) error {
	bucketAndPath, item := asc.resolveBucketPathAndItem(bucket, itemKey)

	// Create AWS config with credentials
	cfgOpts := []func(*config.LoadOptions) error{
		config.WithRegion(DefaultRegion), // default region (some valid region must be mentioned)
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, sessionToken)),
	}
	if endpoint != "" {
		cfgOpts = append(cfgOpts, config.WithBaseEndpoint(endpoint))
	}
	cfg, err := config.LoadDefaultConfig(context.TODO(), cfgOpts...)
	if err != nil {
		return errors.Wrap(err, "Failed to create AWS config")
	}

	// Skip GetBucketLocation when a custom endpoint is set;
	// not all S3-compatible backends implement it.
	if region == "" && endpoint == "" {
		region, err = asc.getBucketRegion(context.TODO(), cfg, bucketAndPath, endpoint)
		if err != nil {
			return errors.Wrap(err, "Failed to get bucket region")
		}
	}
	if region != "" {
		cfg.Region = region
	}

	return asc.download(file, cfg, bucketAndPath, item, endpoint)
}

func (asc *AbstractS3Client) DownloadWithinEC2Instance(file *os.File, bucket, itemKey string) error {
	bucketAndPath, item := asc.resolveBucketPathAndItem(bucket, itemKey)

	// Load default config (will use EC2 instance credentials)
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return errors.Wrap(err, "Failed to create AWS config")
	}
	return asc.download(file, cfg, bucketAndPath, item, "")
}

func (asc *AbstractS3Client) resolveBucketPathAndItem(bucket, itemKey string) (string, string) {
	pathInsideBucket, item := path.Split(filepath.Clean(itemKey))
	bucketAndPath := path.Join(bucket, pathInsideBucket) + "/"
	return bucketAndPath, item
}

// newS3Client forces path-style addressing when a custom endpoint is set,
// which most S3-compatible backends require for sigv4 to match.
func newS3Client(cfg aws.Config, endpoint string) *s3.Client {
	if endpoint == "" {
		return s3.NewFromConfig(cfg)
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
}

func (asc *AbstractS3Client) getBucketRegion(ctx context.Context, cfg aws.Config, bucket, endpoint string) (string, error) {
	s3Client := newS3Client(cfg, endpoint)
	result, err := s3Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return "", err
	}

	// If location is empty, it means us-east-1
	if result.LocationConstraint == "" {
		return DefaultRegion, nil
	}
	return string(result.LocationConstraint), nil
}

func (asc *AbstractS3Client) download(file *os.File, cfg aws.Config, bucketAndPath, item, endpoint string) error {
	s3Client := newS3Client(cfg, endpoint)
	downloader := manager.NewDownloader(s3Client)

	_, err := downloader.Download(context.TODO(), file, &s3.GetObjectInput{
		Bucket: aws.String(bucketAndPath),
		Key:    aws.String(item),
	})
	if err != nil {
		return errors.Wrap(err, "Failed to download file from s3")
	}
	return nil
}

type MockS3Client struct {
	mock.Mock
	FilePath string
}

func (msc *MockS3Client) Download(file *os.File, bucket, itemKey, region, accessKeyID, secretAccessKey, sessionToken, endpoint string) error {
	functionArchiveFileBytes, _ := os.ReadFile(msc.FilePath)

	_ = os.WriteFile(file.Name(), functionArchiveFileBytes, os.FileMode(os.O_RDWR))

	args := msc.Called(file, bucket, itemKey, region, accessKeyID, secretAccessKey, sessionToken, endpoint)
	return args.Error(0)
}

func (msc *MockS3Client) DownloadWithinEC2Instance(file *os.File, bucket, itemKey string) error {
	functionArchiveFileBytes, _ := os.ReadFile(msc.FilePath)

	_ = os.WriteFile(file.Name(), functionArchiveFileBytes, os.FileMode(os.O_RDWR))

	args := msc.Called(file, bucket, itemKey)
	return args.Error(0)
}
