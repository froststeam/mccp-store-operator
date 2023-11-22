package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	FAILED_STATE    = -1
	UPLOADING_STATE = 0
	SUCCESSED_STATE = 1
)

func main() {
	storeName := flag.String("storeName", "", "Storage name for imported data (required)")
	mountPath := flag.String("mountPath", "", "PVC mount path for data import (required)")
	endpoint := flag.String("endpoint", "", "OSS endpoint (required)")
	bucket := flag.String("bucket", "", "OSS bucket (required)")
	accesskey := flag.String("accesskey", "", "OSS accesskey (required)")
	secretkey := flag.String("secretkey", "", "OSS secretkey (required)")
	region := flag.String("region", "us-east-1", "OSS region ")
	disableSSL := flag.Bool("disableSSL", false, "OSS disableSSL ")

	flag.Parse()

	checkFlag(*storeName, "storeName")
	checkFlag(*mountPath, "mountPath")
	checkFlag(*endpoint, "endpoint")
	checkFlag(*bucket, "bucket")
	checkFlag(*accesskey, "accesskey")
	checkFlag(*secretkey, "accesssecret")

	session, err := NewSession(*endpoint, *accesskey, *secretkey, *region, *disableSSL)
	if err != nil {
		panic(err)
	}
	s3Client := s3.New(session)
	downloader := s3manager.NewDownloader(session)

	objects, bucketSize, objectCount, err := ListAllObjectKey(s3Client, *bucket)
	if err != nil {
		panic(err)
	}

	downloadState := &DownloadState{
		TotalBucketSize:  bucketSize,
		TotalObjectCount: objectCount,
		State:            UPLOADING_STATE,
	}
	syncDir := filepath.Join(*mountPath, *storeName)

	SyncAllObjectByKeys(downloader, s3Client, syncDir, *bucket, objects, downloadState)

}

func checkFlag(flag string, name string) {
	if flag == "" {
		fmt.Printf("%s is a required parameter \n", name)
		os.Exit(1)
	}
}
