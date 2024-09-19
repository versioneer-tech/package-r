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

// ObjectSetSpec defines the desired state of ObjectSet
type ObjectSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	FriendlyName string   `json:"friendlyName,omitempty"`
	AllowedRoles []string `json:"allowedRoles,omitempty"`
	SourceName   string   `json:"sourceName"`
	Filter       string   `json:"filter"`
}

// ObjectSetStatus defines the observed state of ObjectSet
type ObjectSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	SecretName string `json:"secretName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ObjectSet is the Schema for the filesets API
type ObjectSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectSetSpec   `json:"spec,omitempty"`
	Status ObjectSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ObjectSetList contains a list of ObjectSet
type ObjectSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectSet `json:"items"`
}

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&ObjectSet{}, &ObjectSetList{})
}
