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

package psbinding

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
)

// BaseReconciler helps implement controller.Reconciler for Binding resources.
type BaseReconciler struct {
	// The GVR of the "primary key" resource
	GVR schema.GroupVersionResource

	Get func(namespace string, name string) (Bindable, error)

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

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*BaseReconciler)(nil)

// Reconcile implements controller.Reconciler
func (r *BaseReconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the resource with this namespace/name.
	original, err := r.Get(namespace, name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("resource %q no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}
	// Don't modify the informers copy.
	resource := original.DeepCopyObject().(Bindable)

	// Reconcile this copy of the resource and then write back any status
	// updates regardless of whether the reconciliation errored out.
	reconcileErr := r.reconcile(ctx, resource)
	if equality.Semantic.DeepEqual(original.GetBindingStatus(), resource.GetBindingStatus()) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if err = r.UpdateStatus(ctx, resource); err != nil {
		logger.Warnw("Failed to update resource status", zap.Error(err))
		r.Recorder.Eventf(resource, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for %q: %v", resource.GetName(), err)
		return err
	}
	if reconcileErr != nil {
		r.Recorder.Event(resource, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *BaseReconciler) reconcile(ctx context.Context, fb Bindable) error {
	if fb.GetDeletionTimestamp() != nil {
		// Check for a DeletionTimestamp.  If present, elide the normal reconcile logic.
		// When a controller needs finalizer handling, it would go here.
		return r.ReconcileDeletion(ctx, fb)
	}
	fb.GetBindingStatus().InitializeConditions()

	if err := r.EnsureFinalizer(ctx, fb); err != nil {
		return err
	}

	if err := r.ReconcileSubject(ctx, fb, fb.Do); err != nil {
		return err
	}

	fb.GetBindingStatus().SetObservedGeneration(fb.GetGeneration())
	return nil
}

func (r *BaseReconciler) EnsureFinalizer(ctx context.Context, fb kmeta.Accessor) error {
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

func (r *BaseReconciler) ReconcileDeletion(ctx context.Context, fb Bindable) error {
	if !r.IsFinalizing(ctx, fb) {
		return nil
	}

	logging.FromContext(ctx).Infof("Removing the binding for %s", fb.GetName())
	if err := r.ReconcileSubject(ctx, fb, fb.Undo); apierrs.IsNotFound(err) {
		// Everything is fine.
	} else if err != nil {
		return err
	}

	return r.RemoveFinalizer(ctx, fb)
}

func (r *BaseReconciler) IsFinalizing(ctx context.Context, fb kmeta.Accessor) bool {
	// If our Finalizer is first, then we are finalizing.
	return len(fb.GetFinalizers()) != 0 && fb.GetFinalizers()[0] == r.GVR.GroupResource().String()
}

func (r *BaseReconciler) RemoveFinalizer(ctx context.Context, fb kmeta.Accessor) error {
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

func (r *BaseReconciler) ReconcileSubject(ctx context.Context, fb Bindable, mutation func(context.Context, *duckv1.WithPod)) error {
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
			fb.GetBindingStatus().MarkBindingUnavailable("SubjectMissing", err.Error())
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
			fb.GetBindingStatus().MarkBindingUnavailable("SubjectMissing", err.Error())
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
		fb.GetBindingStatus().MarkBindingUnavailable("BindingFailed", err.Error())
		return err
	}
	fb.GetBindingStatus().MarkBindingAvailable()
	return nil
}

// Update the Status of the resource.  Caller is responsible for checking
// for semantic differences before calling.
func (r *BaseReconciler) UpdateStatus(ctx context.Context, desired Bindable) error {
	actual, err := r.Get(desired.GetNamespace(), desired.GetName())
	if err != nil {
		logging.FromContext(ctx).Errorf("Error fetching actual: %v", err)
		return err
	}

	ua, err := duck.ToUnstructured(actual)
	if err != nil {
		logging.FromContext(ctx).Errorf("Error converting actual: %v", err)
		return err
	}

	ud, err := duck.ToUnstructured(desired)
	if err != nil {
		logging.FromContext(ctx).Errorf("Error converting desired: %v", err)
		return err
	}

	actualStatus := ua.Object["status"]
	desiredStatus := ud.Object["status"]
	if reflect.DeepEqual(actualStatus, desiredStatus) {
		return nil
	}

	forUpdate := ua
	forUpdate.Object["status"] = desiredStatus
	_, err = r.DynamicClient.Resource(r.GVR).Namespace(desired.GetNamespace()).UpdateStatus(
		forUpdate, metav1.UpdateOptions{})
	return err
}
