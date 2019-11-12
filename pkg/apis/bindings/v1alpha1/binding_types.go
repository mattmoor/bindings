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

	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/tracker"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SinkBinding is a Knative abstraction that encapsulates the interface by which Knative
// components express a desire to have a particular image cached.
type SinkBinding struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the SinkBinding (from the client).
	// +optional
	Spec SinkBindingSpec `json:"spec,omitempty"`

	// Status communicates the observed state of the SinkBinding (from the controller).
	// +optional
	Status SinkBindingStatus `json:"status,omitempty"`
}

// Check that SinkBinding can be validated and defaulted.
var _ apis.Validatable = (*SinkBinding)(nil)
var _ apis.Defaultable = (*SinkBinding)(nil)
var _ kmeta.OwnerRefable = (*SinkBinding)(nil)

// SinkBindingSpec holds the desired state of the SinkBinding (from the client).
type SinkBindingSpec struct {
	// Target holds a reference to the "pod speccable" Kubernetes resource which will
	// have the reference to our sink injected into it.
	Target tracker.Reference `json:"target"`

	// TODO(mattmoor): Add a comment
	Sink duckv1beta1.Destination `json:"sink"`

	// CloudEventOverrides defines overrides to control the output format and
	// modifications of the event sent to the sink.
	// +optional
	CloudEventOverrides *duckv1beta1.CloudEventOverrides `json:"ceOverrides,omitempty"`
}

const (
	// SinkBindingConditionReady is set when the revision is starting to materialize
	// runtime resources, and becomes true when those resources are ready.
	SinkBindingConditionReady = apis.ConditionReady
)

// SinkBindingStatus communicates the observed state of the SinkBinding (from the controller).
type SinkBindingStatus struct {
	duckv1beta1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the
	// Source.
	// +optional
	SinkURI *apis.URL `json:"sinkUri,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SinkBindingList is a list of SinkBinding resources
type SinkBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SinkBinding `json:"items"`
}
