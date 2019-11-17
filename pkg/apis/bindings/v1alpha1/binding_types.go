/*
Copyright 2019 The Knative Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/tracker"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GithubBinding is a Knative abstraction that encapsulates the interface by which Knative
// components express a desire to have a particular image cached.
type GithubBinding struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the GithubBinding (from the client).
	// +optional
	Spec GithubBindingSpec `json:"spec,omitempty"`

	// Status communicates the observed state of the GithubBinding (from the controller).
	// +optional
	Status GithubBindingStatus `json:"status,omitempty"`
}

// Check that GithubBinding can be validated and defaulted.
var _ apis.Validatable = (*GithubBinding)(nil)
var _ apis.Defaultable = (*GithubBinding)(nil)
var _ kmeta.OwnerRefable = (*GithubBinding)(nil)

// GithubBindingSpec holds the desired state of the GithubBinding (from the client).
type GithubBindingSpec struct {
	// Subject holds a reference to the "pod speccable" Kubernetes resource which will
	// be bound with Github secret data.
	Subject tracker.Reference `json:"subject"`

	// Secret holds a reference to a secret containing the Github auth data.
	Secret corev1.LocalObjectReference `json:"secret"`
}

const (
	// GithubBindingConditionReady is set when the binding has been applied to the subjects.
	GithubBindingConditionReady = apis.ConditionReady
)

// GithubBindingStatus communicates the observed state of the GithubBinding (from the controller).
type GithubBindingStatus struct {
	duckv1beta1.Status `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GithubBindingList is a list of GithubBinding resources
type GithubBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GithubBinding `json:"items"`
}
