package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func NewSession(endpoint, accesskey, secretkey, region string, disableSSL bool) (*session.Session, error) {

	awsConfig := aws.Config{
		S3ForcePathStyle: aws.Bool(true),
		Region:           aws.String(region),
		Endpoint:         aws.String(endpoint),
		Credentials:      credentials.NewStaticCredentials(accesskey, secretkey, ""),
		DisableSSL:       aws.Bool(disableSSL),
	}

	session, err := session.NewSession(&awsConfig)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func ListAllObjectKey(s3Client *s3.S3, bucket string) (objects []*s3.Object, size int64, count int64, err error) {

	var allObjects []*s3.Object
	var continuationToken *string
	var totalSize int64 = 0
	var totalCount int64 = 0

	for {
		resp, err := s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			MaxKeys:           aws.Int64(100000),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, 0, 0, err
		}

		for _, object := range resp.Contents {
			totalSize += *object.Size
		}
		totalCount += int64(len(resp.Contents))

		allObjects = append(allObjects, resp.Contents...)

		if !*resp.IsTruncated {
			break
		}

		continuationToken = resp.NextContinuationToken
	}

	return allObjects, totalSize, totalCount, nil
}

func SyncAllObjectByKeys(downloader *s3manager.Downloader, s3Client *s3.S3, path string, bucket string, objects []*s3.Object, downloadState *DownloadState) {
	go func(downloadState *DownloadState) {
		for {
			outputLog(downloadState)
			time.Sleep(time.Second)
		}

	}(downloadState)

	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		downloadState.State = FAILED_STATE
		return
	}

	defer func(downloadState *DownloadState) {
		if downloadState.State != SUCCESSED_STATE {
			os.RemoveAll(path)
			outputLog(downloadState)
		}
	}(downloadState)

	for i, object := range objects {
		objectSize := *object.Size
		objectKey := *object.Key

		downloadState.CurrentObjectSize = objectSize
		downloadState.DownloadedObjectCount = int64(i) + 1
		downloadParams := &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(objectKey),
		}

		objectpath := filepath.Join(path, parseObjectName(objectKey))
		file, err := os.Create(objectpath)
		if err != nil {
			downloadState.State = FAILED_STATE
			return
		}
		writer := &progressWriter{writer: file, downloadState: downloadState}
		if _, err := downloader.Download(writer, downloadParams); err != nil {
			downloadState.State = FAILED_STATE
			return
		}
		downloadState.CurrentDownloadedObjectSize = 0
		outputLog(downloadState)
	}
	downloadState.State = SUCCESSED_STATE
	outputLog(downloadState)
}

func (pw *progressWriter) WriteAt(p []byte, off int64) (int, error) {
	atomic.AddInt64(&pw.downloadState.CurrentDownloadedObjectSize, int64(len(p)))
	atomic.AddInt64(&pw.downloadState.DownloadedBucketSize, int64(len(p)))
	return pw.writer.WriteAt(p, off)
}

func parseObjectName(keyString string) string {
	ss := strings.Split(keyString, "/")
	s := ss[len(ss)-1]
	return s
}

func outputLog(downloadState *DownloadState) {
	state, err := json.Marshal(downloadState)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(state))
}
