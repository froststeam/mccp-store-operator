/*
Copyright 2023.

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

package controller

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	mccpstorev1alpha1 "mccp-store/api/v1alpha1"
)

// ShareStoreReconciler reconciles a ShareStore object
type ShareStoreReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Logger        logr.Logger
	eventRecorder record.EventRecorder
}

var (
	pvcMountPath = "/ceph/"
)

func AddToManager(ctx context.Context, mgr ctrl.Manager) error {
	var (
		controlledType     = &mccpstorev1alpha1.ShareStore{}
		controlledTypeName = reflect.TypeOf(controlledType).Elem().Name()
	)
	r := &ShareStoreReconciler{
		Client:        mgr.GetClient(),
		Logger:        ctrl.Log.WithName("controllers").WithName(controlledTypeName),
		eventRecorder: mgr.GetEventRecorderFor("mccp-store"),
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&mccpstorev1alpha1.ShareStore{}).
		Owns(&batchv1.Job{}).
		Owns(&v1.PersistentVolumeClaim{}).
		WithEventFilter(r.predicate()).
		Complete(r)
}

//+kubebuilder:rbac:groups=store.mccplatform.mthreads.com,resources=sharestores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=store.mccplatform.mthreads.com,resources=sharestores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=store.mccplatform.mthreads.com,resources=sharestores/finalizers,verbs=update
//+kubebuilder:rbac:groups="batch",resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create

func (r *ShareStoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Logger.Info("ShareStoreReconciler Reconcile", "requests", req.String())

	shareStore := &mccpstorev1alpha1.ShareStore{}
	if err := r.Client.Get(context.Background(), client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, shareStore); err != nil {
		r.Logger.Error(err, "failed to get shareStore object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	pvc := &v1.PersistentVolumeClaim{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: shareStore.Name, Namespace: shareStore.Namespace}, pvc, &client.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}
	if pvc.Name == shareStore.Name && pvc.Namespace == shareStore.Namespace {
		r.Logger.Error(fmt.Errorf("failed to create pvc"), "there is already the same PVC", "name", pvc.Name, "namespace", pvc.Namespace)
		return ctrl.Result{}, nil
	}

	pvc = getCephfsPVC(shareStore)
	sycnBucketJob := getSyncBucketJob(shareStore)

	if err := r.Client.Create(context.Background(), pvc, &client.CreateOptions{}); err != nil {
		r.Logger.Error(err, "failed to create cephfs pvc", "name", pvc.Name, "namespace", pvc.Namespace)
		return ctrl.Result{}, err
	}
	r.Logger.Info("successfully create cephfs pvc", "name", pvc.Name, "namespace", pvc.Namespace)

	if err := r.Client.Create(ctx, sycnBucketJob, &client.CreateOptions{}); err != nil {
		r.Logger.Error(err, "failed to create sycnBucketJob", "name", sycnBucketJob.Name, "namespace", sycnBucketJob.Namespace)
		if err = r.Client.Delete(ctx, pvc, &client.DeleteOptions{}); err != nil {
			r.Logger.Error(err, "failed to delete cephfs pvc", "name", pvc.Name, "namespace", pvc.Namespace)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	logEvent(r.eventRecorder, shareStore, v1.EventTypeNormal, "Uploading", "Success to create syncBucketJob")
	r.Logger.Info("successfully create sycnBucketJob", "name", sycnBucketJob.Name, "namespace", sycnBucketJob.Namespace)

	return ctrl.Result{}, nil
}

func logEvent(recorder record.EventRecorder, object runtime.Object, eventType string, reason string, messageFmt string) {
	if recorder != nil {
		recorder.Event(object, eventType, reason, time.Now().Format("2006-01-02 15:04:05")+": "+messageFmt)
	}
}

func (r *ShareStoreReconciler) predicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			shareStore := e.Object.(*mccpstorev1alpha1.ShareStore).DeepCopy()

			pvc := getCephfsPVC(shareStore)
			sycnBucketJob := getSyncBucketJob(shareStore)

			err := r.Client.Delete(context.TODO(), pvc, &client.DeleteOptions{})
			if err != nil {
				r.Logger.Error(err, "failed to delete pvc", "name", pvc.Name, "namespace", pvc.Namespace)
			} else {
				r.Logger.Info("successfully delete pvc of shareStore", "name", pvc.Name, "namespace", pvc.Namespace)
			}

			err = r.Client.Delete(context.TODO(), sycnBucketJob, &client.DeleteOptions{})
			if err != nil {
				r.Logger.Error(err, "failed to delete sycnBucketJob", "name", sycnBucketJob.Name, "namespace", sycnBucketJob.Namespace)
			} else {
				r.Logger.Info("successfully delete pvc of shareStore", "name", sycnBucketJob.Name, "namespace", sycnBucketJob.Namespace)
			}
			return false
		},
	}
}

func getCephfsPVC(shareStore *mccpstorev1alpha1.ShareStore) *v1.PersistentVolumeClaim {

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shareStore.Name,
			Namespace: shareStore.Namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes:      []v1.PersistentVolumeAccessMode{shareStore.Spec.ShareType},
			StorageClassName: &shareStore.Spec.StorageClassName,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: shareStore.Spec.StoreCapacity,
				},
			},
		},
	}

	return pvc
}

func getSyncBucketJob(shareStore *mccpstorev1alpha1.ShareStore) *batchv1.Job {

	cmd := []string{"/sync-bucket",
		"--storeName=" + shareStore.Name,
		"--endpoint=" + shareStore.Spec.OssSpec.Endpoint,
		"--bucket=" + shareStore.Spec.OssSpec.BucketName,
		"--accesskey=" + shareStore.Spec.OssSpec.Accesskey,
		"--secretkey=" + shareStore.Spec.OssSpec.Secretkey,
		"--disableSSL=" + strconv.FormatBool(shareStore.Spec.OssSpec.DisableSSL),
		"--region=" + shareStore.Spec.OssSpec.Region,
		"--mountPath=" + pvcMountPath,
	}

	sycnBucketJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shareStore.Name,
			Namespace: shareStore.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            shareStore.Name,
							Image:           shareStore.Spec.Image,
							ImagePullPolicy: v1.PullAlways,
							Command:         cmd,
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      shareStore.Name,
									ReadOnly:  false,
									MountPath: pvcMountPath,
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: shareStore.Name,
							VolumeSource: v1.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: shareStore.Name,
								},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}

	return sycnBucketJob
}
