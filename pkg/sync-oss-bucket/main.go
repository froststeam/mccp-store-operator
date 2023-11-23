package main

import (
	"context"
	"encoding/json"
	"flag"

	storemccplatformv1alpha1 "mccp-store/api/v1alpha1"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	FAILED_STATE    = -1
	UPLOADING_STATE = 0
	SUCCESSED_STATE = 1
)

var (
	config, _    = CreateKubeConfig()
	clientset, _ = kubernetes.NewForConfig(config)
)

const (
	ShareStoreAPIVersion = "/apis/store.mccplatform.mthreads.com/v1alpha1"
	Resource             = "sharestores"
)

func main() {
	storeName := flag.String("storeName", "", "Storage name for imported data (required)")
	storeNamespace := flag.String("shareStoreNamesoace", "", "ShareStore Name (required)")
	endpoint := flag.String("endpoint", "", "OSS endpoint (required)")
	bucket := flag.String("bucket", "", "OSS bucket (required)")
	accesskey := flag.String("accesskey", "", "OSS accesskey (required)")
	secretkey := flag.String("secretkey", "", "OSS secretkey (required)")
	region := flag.String("region", "us-east-1", "OSS region ")
	disableSSL := flag.Bool("disableSSL", false, "OSS disableSSL ")
	flag.Parse()

	CheckFlag(*storeName, "storeName")
	CheckFlag(*storeNamespace, "shareStoreNamespace")
	CheckFlag(*endpoint, "endpoint")
	CheckFlag(*bucket, "bucket")
	CheckFlag(*accesskey, "accesskey")
	CheckFlag(*secretkey, "accesssecret")

	shareStore := &storemccplatformv1alpha1.ShareStore{}
	if err := QueryMccpShareStore(shareStore, *storeName, *storeNamespace); err != nil {
		klog.Error(err)
		panic(err)
	}

	session, err := NewSession(*endpoint, *accesskey, *secretkey, *region, *disableSSL)
	if err != nil {
		shareStore.Status.State = storemccplatformv1alpha1.FailedState
		ExecuteUpdateShareStore(shareStore)
		klog.Error(err)
		panic(err)
	}

	s3Client := s3.New(session)
	downloader := s3manager.NewDownloader(session)

	objects, bucketSize, objectCount, err := ListAllObjectKey(s3Client, *bucket)
	if err != nil {
		shareStore.Status.State = storemccplatformv1alpha1.FailedState
		ExecuteUpdateShareStore(shareStore)
		klog.Error(err)
		panic(err)
	}

	downloadState := &DownloadState{
		TotalBucketSize:  bucketSize,
		TotalObjectCount: objectCount,
		State:            UPLOADING_STATE,
	}

	SyncAllObjectByKeys(downloader, s3Client, *storeName, *bucket, objects, downloadState, shareStore)
}

func QueryMccpShareStore(mccpShareStore *storemccplatformv1alpha1.ShareStore, name, namespace string) error {
	shareStoreRaw, err := clientset.CoreV1().RESTClient().Get().
		AbsPath(ShareStoreAPIVersion).
		Name(name).
		Namespace(namespace).
		Resource(Resource).
		DoRaw(context.TODO())

	err = json.Unmarshal(shareStoreRaw, &mccpShareStore)
	if err != nil {
		return err
	}
	return nil
}

func ExecuteUpdateShareStore(mccpShareStore *storemccplatformv1alpha1.ShareStore) error {
	body, err := json.Marshal(mccpShareStore)
	if err != nil {
		return err
	}

	_, err = clientset.CoreV1().RESTClient().Put().
		AbsPath(ShareStoreAPIVersion).
		Name(mccpShareStore.Name).
		Namespace(mccpShareStore.Namespace).
		Resource(Resource).
		Body(body).
		DoRaw(context.TODO())

	return err
}
