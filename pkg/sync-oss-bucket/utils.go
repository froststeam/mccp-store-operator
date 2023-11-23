package main

import (
	"encoding/json"
	"fmt"
	storemccplatformv1alpha1 "mccp-store/api/v1alpha1"
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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
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

func SyncAllObjectByKeys(downloader *s3manager.Downloader, s3Client *s3.S3, path string, bucket string, objects []*s3.Object, downloadState *DownloadState, shareStore *storemccplatformv1alpha1.ShareStore) {
	go func(downloadState *DownloadState) {
		for {
			shareStore.Status.State = storemccplatformv1alpha1.SyncingState
			if downloadState.TotalBucketSize != 0 {
				shareStore.Status.Progress = fmt.Sprint(downloadState.DownloadedBucketSize/downloadState.TotalBucketSize) + "%"
			}
			ExecuteUpdateShareStore(shareStore)
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
	shareStore.Status.State = storemccplatformv1alpha1.CompletedState
	shareStore.Status.Progress = "100%"
	ExecuteUpdateShareStore(shareStore)
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

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func CreateKubeConfig() (*rest.Config, error) {
	kubeConfigPath := ""
	if home := homedir.HomeDir(); home != "" {
		kubeConfigPath = filepath.Join(home, ".kube", "config")
	}
	fileExist, err := PathExists(kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("justify kubeConfigPath exist err,err:%v", err)
	}

	if fileExist {
		// local run
		config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return nil, err
		}
		return config, nil
	} else {
		// pod run
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		return config, nil
	}
}

func CheckFlag(flag string, name string) {
	if flag == "" {
		fmt.Printf("%s is a required parameter \n", name)
		os.Exit(1)
	}
}
