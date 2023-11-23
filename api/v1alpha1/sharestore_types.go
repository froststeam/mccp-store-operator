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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SyncingState   = "syncing"   // The upload job is progressing
	CompletedState = "completed" // The upload task is successfully executed
	FailedState    = "failed"    // The upload task failed to be executed
)

type ShareStoreSpec struct {
	Image            string                        `json:"image,omitempty"`
	StorageClassName string                        `json:"storageClassName,omitempty"`
	ShareType        v1.PersistentVolumeAccessMode `json:"shareType,omitempty"`
	DefaultMountPath string                        `json:"defaultMountPath,omitempty"`
	StoreCapacity    resource.Quantity             `json:"storeCapacity,omitempty"`
	OssSpec          OssSpec                       `json:"ossSpec,omitempty"`
}

type OssSpec struct {
	Endpoint   string `json:"endpoint,omitempty"`
	BucketName string `json:"bucketName,omitempty"`
	Accesskey  string `json:"accesskey,omitempty"`
	Secretkey  string `json:"secretkey,omitempty"`
	Region     string `json:"region,omitempty"`
	DisableSSL bool   `json:"disableSSL,omitempty"`
}

type ShareStoreStatus struct {
	Progress string `json:"progress,omitempty"`
	State    string `json:"state,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type ShareStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ShareStoreSpec   `json:"spec,omitempty"`
	Status ShareStoreStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type ShareStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ShareStore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ShareStore{}, &ShareStoreList{})
}
