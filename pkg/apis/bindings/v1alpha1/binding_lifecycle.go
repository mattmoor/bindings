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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

var condSet = apis.NewLivingConditionSet()

// GetGroupVersionKind implements kmeta.OwnerRefable
func (fb *SinkBinding) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("SinkBinding")
}

func (fbs *SinkBindingStatus) InitializeConditions() {
	condSet.Manage(fbs).InitializeConditions()
}

func (fbs *SinkBindingStatus) MarkBindingUnavailable(reason, message string) {
	condSet.Manage(fbs).MarkFalse(
		SinkBindingConditionReady,
		reason, message)
}

func (fbs *SinkBindingStatus) MarkBindingAvailable() {
	condSet.Manage(fbs).MarkTrue(SinkBindingConditionReady)
}

func (fb *SinkBinding) Do(ps *PodSpeccable, uri string) {
	spec := ps.Spec.Template.Spec
	for i, c := range spec.Containers {
		found := false
		for j, ev := range c.Env {
			if ev.Name == "FOO" {
				spec.Containers[i].Env[j].Value = "Awesomesauce"
				found = true
				break
			}
		}
		if !found {
			spec.Containers[i].Env = append(spec.Containers[i].Env, corev1.EnvVar{
				Name:  "SINK",
				Value: uri,
			})
		}
	}
}

func (fb *SinkBinding) Undo(ps *PodSpeccable) {
	spec := ps.Spec.Template.Spec
	for i, c := range spec.Containers {
		for j, ev := range c.Env {
			if ev.Name == "SINK" {
				spec.Containers[i].Env = append(spec.Containers[i].Env[:j], spec.Containers[i].Env[j+1:]...)
				break
			}
		}
	}
}
