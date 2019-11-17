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

	"github.com/mattmoor/bindings/pkg/github"
)

var condSet = apis.NewLivingConditionSet()

// GetGroupVersionKind implements kmeta.OwnerRefable
func (fb *GithubBinding) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("GithubBinding")
}

func (fbs *GithubBindingStatus) InitializeConditions() {
	condSet.Manage(fbs).InitializeConditions()
}

func (fbs *GithubBindingStatus) MarkBindingUnavailable(reason, message string) {
	condSet.Manage(fbs).MarkFalse(
		GithubBindingConditionReady,
		reason, message)
}

func (fbs *GithubBindingStatus) MarkBindingAvailable() {
	condSet.Manage(fbs).MarkTrue(GithubBindingConditionReady)
}

func (fb *GithubBinding) Do(ps *PodSpeccable) {

	// First undo so that we can just unconditionally append below.
	fb.Undo(ps)

	// Make sure the PodSpec has a Volume like this:
	volume := corev1.Volume{
		Name: github.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: fb.Spec.Secret.Name,
			},
		},
	}
	ps.Spec.Template.Spec.Volumes = append(ps.Spec.Template.Spec.Volumes, volume)

	// Make sure that each [init]container in the PodSpec has a VolumeMount like this:
	volumeMount := corev1.VolumeMount{
		Name:      github.VolumeName,
		ReadOnly:  true,
		MountPath: github.MountPath,
	}
	spec := ps.Spec.Template.Spec
	for i := range spec.InitContainers {
		spec.InitContainers[i].VolumeMounts = append(spec.InitContainers[i].VolumeMounts, volumeMount)
	}
	for i := range spec.Containers {
		spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts, volumeMount)
	}
}

func (fb *GithubBinding) Undo(ps *PodSpeccable) {
	spec := ps.Spec.Template.Spec

	// Make sure the PodSpec does NOT have the github volume.
	for i, v := range spec.Volumes {
		if v.Name == github.VolumeName {
			spec.Volumes = append(spec.Volumes[:i], spec.Volumes[i+1:]...)
			break
		}
	}

	// Make sure that none of the [init]containers have the github volume mount
	for i, c := range spec.InitContainers {
		for j, ev := range c.VolumeMounts {
			if ev.Name == github.VolumeName {
				spec.InitContainers[i].VolumeMounts = append(spec.InitContainers[i].VolumeMounts[:j], spec.InitContainers[i].VolumeMounts[j+1:]...)
				break
			}
		}
	}
	for i, c := range spec.Containers {
		for j, ev := range c.VolumeMounts {
			if ev.Name == github.VolumeName {
				spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts[:j], spec.Containers[i].VolumeMounts[j+1:]...)
				break
			}
		}
	}
}
