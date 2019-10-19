/*
Copyright 2019 The Knative Authors

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodSpeccable is a duck type that the resources referenced by a
// binding's Target must implement.
type PodSpeccable struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PodSpeccableSpec `json:"spec"`
}

// PodSpeccableSpec is the specification for the desired state of a
// PodSpeccable (or at least our shared portion).
type PodSpeccableSpec struct {
	Template corev1.PodTemplateSpec `json:"template"`
}

var _ duck.Populatable = (*PodSpeccable)(nil)
var _ duck.Implementable = (*PodSpeccable)(nil)
var _ apis.Listable = (*PodSpeccable)(nil)

// GetFullType implements duck.Implementable
func (*PodSpeccable) GetFullType() duck.Populatable {
	return &PodSpeccable{}
}

// Populate implements duck.Populatable
func (t *PodSpeccable) Populate() {
	t.Spec = PodSpeccableSpec{
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "container-name",
					Image: "container-image:latest",
				}},
			},
		},
	}
}

// GetListType implements apis.Listable
func (*PodSpeccable) GetListType() runtime.Object {
	return &PodSpeccableList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodSpeccableList is a list of PodSpeccable resources
type PodSpeccableList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PodSpeccable `json:"items"`
}
