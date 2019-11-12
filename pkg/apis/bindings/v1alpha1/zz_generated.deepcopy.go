// +build !ignore_autogenerated

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
	apis "knative.dev/pkg/apis"
	v1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodSpeccable) DeepCopyInto(out *PodSpeccable) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodSpeccable.
func (in *PodSpeccable) DeepCopy() *PodSpeccable {
	if in == nil {
		return nil
	}
	out := new(PodSpeccable)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PodSpeccable) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodSpeccableList) DeepCopyInto(out *PodSpeccableList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PodSpeccable, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodSpeccableList.
func (in *PodSpeccableList) DeepCopy() *PodSpeccableList {
	if in == nil {
		return nil
	}
	out := new(PodSpeccableList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PodSpeccableList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodSpeccableSpec) DeepCopyInto(out *PodSpeccableSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodSpeccableSpec.
func (in *PodSpeccableSpec) DeepCopy() *PodSpeccableSpec {
	if in == nil {
		return nil
	}
	out := new(PodSpeccableSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SinkBinding) DeepCopyInto(out *SinkBinding) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SinkBinding.
func (in *SinkBinding) DeepCopy() *SinkBinding {
	if in == nil {
		return nil
	}
	out := new(SinkBinding)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SinkBinding) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SinkBindingList) DeepCopyInto(out *SinkBindingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SinkBinding, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SinkBindingList.
func (in *SinkBindingList) DeepCopy() *SinkBindingList {
	if in == nil {
		return nil
	}
	out := new(SinkBindingList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SinkBindingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SinkBindingSpec) DeepCopyInto(out *SinkBindingSpec) {
	*out = *in
	in.Target.DeepCopyInto(&out.Target)
	in.Sink.DeepCopyInto(&out.Sink)
	if in.CloudEventOverrides != nil {
		in, out := &in.CloudEventOverrides, &out.CloudEventOverrides
		*out = new(v1beta1.CloudEventOverrides)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SinkBindingSpec.
func (in *SinkBindingSpec) DeepCopy() *SinkBindingSpec {
	if in == nil {
		return nil
	}
	out := new(SinkBindingSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SinkBindingStatus) DeepCopyInto(out *SinkBindingStatus) {
	*out = *in
	in.Status.DeepCopyInto(&out.Status)
	if in.SinkURI != nil {
		in, out := &in.SinkURI, &out.SinkURI
		*out = new(apis.URL)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SinkBindingStatus.
func (in *SinkBindingStatus) DeepCopy() *SinkBindingStatus {
	if in == nil {
		return nil
	}
	out := new(SinkBindingStatus)
	in.DeepCopyInto(out)
	return out
}