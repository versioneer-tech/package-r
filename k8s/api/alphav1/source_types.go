/*
Copyright 2024.

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

package alphav1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Access provides details to access the items of the source
type Access struct {
	BucketName   string `json:"bucketName,omitempty"`
	BucketPrefix string `json:"bucketPrefix,omitempty"`
	SecretName   string `json:"secretName,omitempty"`
}

// Share provides details to share the items of the source
type Share struct {
	BucketName   string `json:"bucketName,omitempty"`
	BucketPrefix string `json:"bucketPrefix,omitempty"`
	SecretName   string `json:"secretName,omitempty"`
}

// SourceSpec defines the desired state of Source
type SourceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	FriendlyName string   `json:"friendlyName,omitempty"`
	AllowedRoles []string `json:"allowedRoles,omitempty"`
	SubPath      string   `json:"subPath,omitempty"`
	Mount        string   `json:"mount,omitempty"`
	Access       Access   `json:"access,omitempty"`
	Share        Share    `json:"share,omitempty"`
}

// SourceStatus defines the observed state of Source
type SourceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Error string `json:"error,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Source is the Schema for the sources API
type Source struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SourceSpec   `json:"spec,omitempty"`
	Status SourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SourceList contains a list of Source
type SourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Source `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&Source{}, &SourceList{})
}
