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

package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/mattmoor/bindings/pkg/webhook/psbinding"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
)

// Base helps implement controller.Reconciler for Binding resources.
type Base struct {
	// The GVR of the "primary key" resource
	GVR schema.GroupVersionResource

	Get func(namespace string, name string) (*unstructured.Unstructured, error)

	// DynamicClient is used to patch subjects.
	DynamicClient dynamic.Interface

	// Factory is used for producing listers for the object references we encounter.
	Factory duck.InformerFactory

	// The tracker builds an index of what resources are watching other
	// resources so that we can immediately react to changes to changes in
	// tracked resources.
	Tracker tracker.Interface

	// Recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	Recorder record.EventRecorder
}

func (r *Base) EnsureFinalizer(ctx context.Context, fb kmeta.Accessor) error {
	finalizers := sets.NewString(fb.GetFinalizers()...)
	if finalizers.Has(r.GVR.GroupResource().String()) {
		return nil
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      append(fb.GetFinalizers(), r.GVR.GroupResource().String()),
			"resourceVersion": fb.GetResourceVersion(),
		},
	}
	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}

	_, err = r.DynamicClient.Resource(r.GVR).Namespace(fb.GetNamespace()).Patch(fb.GetName(),
		types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func (r *Base) IsFinalizing(ctx context.Context, fb kmeta.Accessor) bool {
	// If our Finalizer is first, then we are finalizing.
	return len(fb.GetFinalizers()) != 0 && fb.GetFinalizers()[0] == r.GVR.GroupResource().String()
}

func (r *Base) RemoveFinalizer(ctx context.Context, fb kmeta.Accessor) error {
	logger := logging.FromContext(ctx)
	logger.Info("Removing Finalizer")

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      fb.GetFinalizers()[1:],
			"resourceVersion": fb.GetResourceVersion(),
		},
	}
	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}

	_, err = r.DynamicClient.Resource(r.GVR).Namespace(fb.GetNamespace()).Patch(fb.GetName(),
		types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func (r *Base) ReconcileSubject(ctx context.Context, fb psbinding.Bindable, mutation func(context.Context, *duckv1.WithPod)) error {
	logger := logging.FromContext(ctx)

	subject := fb.GetSubject()
	if err := r.Tracker.TrackReference(subject, fb); err != nil {
		logger.Errorf("Error tracking subject %v: %v", subject, err)
		return err
	}

	// Determine the GroupVersionResource of the subject reference
	gv, err := schema.ParseGroupVersion(subject.APIVersion)
	if err != nil {
		logger.Errorf("Error parsing GroupVersion %v: %v", subject.APIVersion, err)
		return err
	}
	gvr := apis.KindToResource(gv.WithKind(subject.Kind))

	_, lister, err := r.Factory.Get(gvr)
	if err != nil {
		return fmt.Errorf("error getting a lister for resource '%+v': %w", gvr, err)
	}

	var referents []*duckv1.WithPod
	if subject.Name != "" {
		psObj, err := lister.ByNamespace(subject.Namespace).Get(subject.Name)
		if apierrs.IsNotFound(err) {
			fb.MarkBindingUnavailable("SubjectMissing", err.Error())
			return err
		} else if err != nil {
			return fmt.Errorf("error fetching Pod Speccable %v: %w", subject, err)
		}
		referents = append(referents, psObj.(*duckv1.WithPod))
	} else {
		selector, err := metav1.LabelSelectorAsSelector(subject.Selector)
		if err != nil {
			return err
		}
		psObjs, err := lister.ByNamespace(subject.Namespace).List(selector)
		if apierrs.IsNotFound(err) {
			fb.MarkBindingUnavailable("SubjectMissing", err.Error())
			return err
		} else if err != nil {
			return fmt.Errorf("error fetching Pod Scalable %v: %w", subject, err)
		}
		for _, psObj := range psObjs {
			referents = append(referents, psObj.(*duckv1.WithPod))
		}
	}

	eg := errgroup.Group{}
	for _, ps := range referents {
		ps := ps
		eg.Go(func() error {
			// Do the binding to the pod speccable.
			orig := ps.DeepCopy()
			mutation(ctx, ps)

			// If nothing changed, then bail early.
			if equality.Semantic.DeepEqual(orig, ps) {
				return nil
			}

			patch, err := duck.CreatePatch(orig, ps)
			if err != nil {
				return err
			}
			patchBytes, err := patch.MarshalJSON()
			if err != nil {
				return err
			}

			logger.Infof("Applying patch: %s", string(patchBytes))

			// TODO(mattmoor): This might fail because a binding changed after
			// a Job started or completed, which can be fine.
			_, err = r.DynamicClient.Resource(gvr).Namespace(ps.Namespace).Patch(
				ps.Name, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
			if err != nil {
				return errors.Wrapf(err, "failed binding subject "+ps.Name)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		fb.MarkBindingUnavailable("BindingFailed", err.Error())
		return err
	}
	fb.MarkBindingAvailable()
	return nil
}

// Update the Status of the resource.  Caller is responsible for checking
// for semantic differences before calling.
func (r *Base) UpdateStatus(ctx context.Context, desired psbinding.Bindable) error {
	actual, err := r.Get(desired.GetNamespace(), desired.GetName())
	if err != nil {
		logging.FromContext(ctx).Errorf("Error fetching actual: %v", err)
		return err
	}

	ud, err := psbinding.ToUnstructured(desired)
	if err != nil {
		logging.FromContext(ctx).Errorf("Error converting desired: %v", err)
		return err
	}

	actualStatus := actual.Object["status"]
	desiredStatus := ud.Object["status"]
	if reflect.DeepEqual(actualStatus, desiredStatus) {
		return nil
	}

	forUpdate := actual
	forUpdate.Object["status"] = desiredStatus
	_, err = r.DynamicClient.Resource(r.GVR).Namespace(desired.GetNamespace()).UpdateStatus(
		forUpdate, metav1.UpdateOptions{})
	return err
}
