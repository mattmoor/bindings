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
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracker"

	"github.com/mattmoor/bindings/pkg/github"
	"github.com/mattmoor/bindings/pkg/slack"
	"github.com/mattmoor/bindings/pkg/twitter"
)

const (
	// GithubBindingConditionReady is set when the binding has been applied to the subjects.
	GithubBindingConditionReady = apis.ConditionReady
)

var ghCondSet = apis.NewLivingConditionSet()

// GetGroupVersionKind implements kmeta.OwnerRefable
func (fb *GithubBinding) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("GithubBinding")
}

// GetSubject implements Bindable
func (fb *GithubBinding) GetSubject() tracker.Reference {
	return fb.Spec.Subject
}

func (fbs *GithubBindingStatus) InitializeConditions() {
	ghCondSet.Manage(fbs).InitializeConditions()
}

func (fb *GithubBinding) MarkBindingUnavailable(reason, message string) {
	fb.Status.MarkBindingUnavailable(reason, message)
}

func (fb *GithubBinding) MarkBindingAvailable() {
	fb.Status.MarkBindingAvailable()
}

func (fbs *GithubBindingStatus) MarkBindingUnavailable(reason, message string) {
	ghCondSet.Manage(fbs).MarkFalse(
		GithubBindingConditionReady, reason, message)
}

func (fbs *GithubBindingStatus) MarkBindingAvailable() {
	ghCondSet.Manage(fbs).MarkTrue(GithubBindingConditionReady)
}

func (fb *GithubBinding) Do(ctx context.Context, ps *duckv1.WithPod) {

	// First undo so that we can just unconditionally append below.
	fb.Undo(ctx, ps)

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

func (fb *GithubBinding) Undo(ctx context.Context, ps *duckv1.WithPod) {
	spec := ps.Spec.Template.Spec

	// Make sure the PodSpec does NOT have the github volume.
	for i, v := range spec.Volumes {
		if v.Name == github.VolumeName {
			ps.Spec.Template.Spec.Volumes = append(spec.Volumes[:i], spec.Volumes[i+1:]...)
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

const (
	// SlackBindingConditionReady is set when the binding has been applied to the subjects.
	SlackBindingConditionReady = apis.ConditionReady
)

var slackCondSet = apis.NewLivingConditionSet()

// GetGroupVersionKind implements kmeta.OwnerRefable
func (fb *SlackBinding) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("SlackBinding")
}

// GetSubject implements Bindable
func (fb *SlackBinding) GetSubject() tracker.Reference {
	return fb.Spec.Subject
}

func (fbs *SlackBindingStatus) InitializeConditions() {
	slackCondSet.Manage(fbs).InitializeConditions()
}

func (fb *SlackBinding) MarkBindingUnavailable(reason, message string) {
	fb.Status.MarkBindingUnavailable(reason, message)
}

func (fb *SlackBinding) MarkBindingAvailable() {
	fb.Status.MarkBindingAvailable()
}

func (fbs *SlackBindingStatus) MarkBindingUnavailable(reason, message string) {
	slackCondSet.Manage(fbs).MarkFalse(
		SlackBindingConditionReady, reason, message)
}

func (fbs *SlackBindingStatus) MarkBindingAvailable() {
	slackCondSet.Manage(fbs).MarkTrue(SlackBindingConditionReady)
}

func (fb *SlackBinding) Do(ctx context.Context, ps *duckv1.WithPod) {

	// First undo so that we can just unconditionally append below.
	fb.Undo(ctx, ps)

	// Make sure the PodSpec has a Volume like this:
	volume := corev1.Volume{
		Name: slack.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: fb.Spec.Secret.Name,
			},
		},
	}
	ps.Spec.Template.Spec.Volumes = append(ps.Spec.Template.Spec.Volumes, volume)

	// Make sure that each [init]container in the PodSpec has a VolumeMount like this:
	volumeMount := corev1.VolumeMount{
		Name:      slack.VolumeName,
		ReadOnly:  true,
		MountPath: slack.MountPath,
	}
	spec := ps.Spec.Template.Spec
	for i := range spec.InitContainers {
		spec.InitContainers[i].VolumeMounts = append(spec.InitContainers[i].VolumeMounts, volumeMount)
	}
	for i := range spec.Containers {
		spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts, volumeMount)
	}
}

func (fb *SlackBinding) Undo(ctx context.Context, ps *duckv1.WithPod) {
	spec := ps.Spec.Template.Spec

	// Make sure the PodSpec does NOT have the slack volume.
	for i, v := range spec.Volumes {
		if v.Name == slack.VolumeName {
			ps.Spec.Template.Spec.Volumes = append(spec.Volumes[:i], spec.Volumes[i+1:]...)
			break
		}
	}

	// Make sure that none of the [init]containers have the slack volume mount
	for i, c := range spec.InitContainers {
		for j, ev := range c.VolumeMounts {
			if ev.Name == slack.VolumeName {
				spec.InitContainers[i].VolumeMounts = append(spec.InitContainers[i].VolumeMounts[:j], spec.InitContainers[i].VolumeMounts[j+1:]...)
				break
			}
		}
	}
	for i, c := range spec.Containers {
		for j, ev := range c.VolumeMounts {
			if ev.Name == slack.VolumeName {
				spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts[:j], spec.Containers[i].VolumeMounts[j+1:]...)
				break
			}
		}
	}
}

const (
	// TwitterBindingConditionReady is set when the binding has been applied to the subjects.
	TwitterBindingConditionReady = apis.ConditionReady
)

var twitterCondSet = apis.NewLivingConditionSet()

// GetGroupVersionKind implements kmeta.OwnerRefable
func (fb *TwitterBinding) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("TwitterBinding")
}

// GetSubject implements Bindable
func (fb *TwitterBinding) GetSubject() tracker.Reference {
	return fb.Spec.Subject
}

func (fbs *TwitterBindingStatus) InitializeConditions() {
	twitterCondSet.Manage(fbs).InitializeConditions()
}

func (fb *TwitterBinding) MarkBindingUnavailable(reason, message string) {
	fb.Status.MarkBindingUnavailable(reason, message)
}

func (fb *TwitterBinding) MarkBindingAvailable() {
	fb.Status.MarkBindingAvailable()
}

func (fbs *TwitterBindingStatus) MarkBindingUnavailable(reason, message string) {
	twitterCondSet.Manage(fbs).MarkFalse(
		TwitterBindingConditionReady, reason, message)
}

func (fbs *TwitterBindingStatus) MarkBindingAvailable() {
	twitterCondSet.Manage(fbs).MarkTrue(TwitterBindingConditionReady)
}

func (fb *TwitterBinding) Do(ctx context.Context, ps *duckv1.WithPod) {

	// First undo so that we can just unconditionally append below.
	fb.Undo(ctx, ps)

	// Make sure the PodSpec has a Volume like this:
	volume := corev1.Volume{
		Name: twitter.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: fb.Spec.Secret.Name,
			},
		},
	}
	ps.Spec.Template.Spec.Volumes = append(ps.Spec.Template.Spec.Volumes, volume)

	// Make sure that each [init]container in the PodSpec has a VolumeMount like this:
	volumeMount := corev1.VolumeMount{
		Name:      twitter.VolumeName,
		ReadOnly:  true,
		MountPath: twitter.MountPath,
	}
	spec := ps.Spec.Template.Spec
	for i := range spec.InitContainers {
		spec.InitContainers[i].VolumeMounts = append(spec.InitContainers[i].VolumeMounts, volumeMount)
	}
	for i := range spec.Containers {
		spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts, volumeMount)
	}
}

func (fb *TwitterBinding) Undo(ctx context.Context, ps *duckv1.WithPod) {
	spec := ps.Spec.Template.Spec

	// Make sure the PodSpec does NOT have the twitter volume.
	for i, v := range spec.Volumes {
		if v.Name == twitter.VolumeName {
			ps.Spec.Template.Spec.Volumes = append(spec.Volumes[:i], spec.Volumes[i+1:]...)
			break
		}
	}

	// Make sure that none of the [init]containers have the twitter volume mount
	for i, c := range spec.InitContainers {
		for j, ev := range c.VolumeMounts {
			if ev.Name == twitter.VolumeName {
				spec.InitContainers[i].VolumeMounts = append(spec.InitContainers[i].VolumeMounts[:j], spec.InitContainers[i].VolumeMounts[j+1:]...)
				break
			}
		}
	}
	for i, c := range spec.Containers {
		for j, ev := range c.VolumeMounts {
			if ev.Name == twitter.VolumeName {
				spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts[:j], spec.Containers[i].VolumeMounts[j+1:]...)
				break
			}
		}
	}
}
