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

package foobinding

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/mattmoor/foo-binding/pkg/apis/bindings/v1alpha1"
	clientset "github.com/mattmoor/foo-binding/pkg/client/clientset/versioned"
	listers "github.com/mattmoor/foo-binding/pkg/client/listers/bindings/v1alpha1"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
)

// Reconciler implements controller.Reconciler for FooBinding resources.
type Reconciler struct {
	// Client is used to write back status updates.
	Client clientset.Interface

	// DynamicClient is used to patch target objects.
	DynamicClient dynamic.Interface

	// Listers index properties about resources
	Lister listers.FooBindingLister

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
var _ controller.Reconciler = (*Reconciler)(nil)

var (
	bindingResource  = v1alpha1.Resource("bindings")
	bindingFinalizer = bindingResource.String()
)

// Reconcile implements controller.Reconciler
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// If our controller has configuration state, we'd "freeze" it and
	// attach the frozen configuration to the context.
	//    ctx = r.configStore.ToContext(ctx)

	// Get the resource with this namespace/name.
	original, err := r.Lister.FooBindings(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("resource %q no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}
	// Don't modify the informers copy.
	resource := original.DeepCopy()

	// Reconcile this copy of the resource and then write back any status
	// updates regardless of whether the reconciliation errored out.
	reconcileErr := r.reconcile(ctx, resource)
	if equality.Semantic.DeepEqual(original.Status, resource.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err = r.updateStatus(resource); err != nil {
		logger.Warnw("Failed to update resource status", zap.Error(err))
		r.Recorder.Eventf(resource, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for %q: %v", resource.Name, err)
		return err
	}
	if reconcileErr != nil {
		r.Recorder.Event(resource, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, fb *v1alpha1.FooBinding) error {
	if fb.GetDeletionTimestamp() != nil {
		// Check for a DeletionTimestamp.  If present, elide the normal reconcile logic.
		// When a controller needs finalizer handling, it would go here.

		return r.reconcileDeletion(ctx, fb)
	}
	fb.Status.InitializeConditions()

	if err := r.ensureFinalizer(ctx, fb); err != nil {
		return err
	}

	if err := r.reconcileTarget(ctx, fb, fb.Do); err != nil {
		return err
	}

	fb.Status.ObservedGeneration = fb.Generation
	return nil
}

func (r *Reconciler) ensureFinalizer(ctx context.Context, fb *v1alpha1.FooBinding) error {
	finalizers := sets.NewString(fb.GetFinalizers()...)
	if finalizers.Has(bindingFinalizer) {
		return nil
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      append(fb.GetFinalizers(), bindingFinalizer),
			"resourceVersion": fb.GetResourceVersion(),
		},
	}
	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}

	_, err = r.Client.BindingsV1alpha1().FooBindings(fb.Namespace).Patch(fb.Name,
		types.MergePatchType, patch)
	return err
}

func (r *Reconciler) reconcileDeletion(ctx context.Context, fb *v1alpha1.FooBinding) error {
	logger := logging.FromContext(ctx)

	// If our Finalizer is first, delete the `Servers` from Gateway for this Ingress,
	// and remove the finalizer.
	if len(fb.GetFinalizers()) == 0 || fb.GetFinalizers()[0] != bindingFinalizer {
		return nil
	}

	logger.Infof("Removing the binding for %s", fb.Name)
	if err := r.reconcileTarget(ctx, fb, fb.Undo); err != nil {
		return err
	}

	// Update the Ingress to remove the Finalizer.
	logger.Info("Removing Finalizer")
	fb.SetFinalizers(fb.GetFinalizers()[1:])
	_, err := r.Client.BindingsV1alpha1().FooBindings(fb.Namespace).Update(fb)
	return err
}

func (r *Reconciler) reconcileTarget(ctx context.Context, fb *v1alpha1.FooBinding, mutation func(*v1alpha1.PodSpeccable)) error {
	logger := logging.FromContext(ctx)

	if err := r.Tracker.Track(fb.Spec.Target, fb); err != nil {
		logger.Errorf("Error tracking target %v: %v", fb.Spec.Target, err)
		return err
	}

	// Determine the GroupVersionResource of the target reference
	gv, err := schema.ParseGroupVersion(fb.Spec.Target.APIVersion)
	if err != nil {
		logger.Errorf("Error parsing GroupVersion %v: %v", fb.Spec.Target.APIVersion, err)
		return err
	}
	gvr := apis.KindToResource(gv.WithKind(fb.Spec.Target.Kind))

	_, lister, err := r.Factory.Get(gvr)
	if err != nil {
		return fmt.Errorf("error getting a lister for resource '%+v': %w", gvr, err)
	}

	psObj, err := lister.ByNamespace(fb.Spec.Target.Namespace).Get(fb.Spec.Target.Name)
	if apierrs.IsNotFound(err) {
		fb.Status.MarkBindingUnavailable("TargetMissing", err.Error())
		return err
	} else if err != nil {
		return fmt.Errorf("error fetching Pod Scalable %v: %w", fb.Spec.Target, err)
	}
	ps := psObj.(*v1alpha1.PodSpeccable)

	// Do the binding to the pod speccable.
	orig := ps.DeepCopy()
	mutation(ps)

	// TODO(mattmoor): We won't be able to patch all PodSpeccable resources,
	// e.g. Job, Build which are immutable, so quickly reconciling things is
	// insufficient, we also need the capability to apply the above in our
	// webhook so that the binding happens before things are committed to etcd.
	if !equality.Semantic.DeepEqual(orig, ps) {
		patch, err := duck.CreatePatch(orig, ps)
		if err != nil {
			return err
		}
		patchBytes, err := patch.MarshalJSON()
		if err != nil {
			return err
		}

		logger.Infof("Applying patch: %s", string(patchBytes))

		_, err = r.DynamicClient.Resource(gvr).Namespace(ps.Namespace).Patch(ps.Name, types.JSONPatchType,
			patchBytes, metav1.PatchOptions{})
		if err != nil {
			fb.Status.MarkBindingUnavailable("PatchFailed", err.Error())
			return errors.Wrapf(err, "failed binding target "+ps.Name)
		}
	}

	fb.Status.MarkBindingAvailable()
	return nil
}

// Update the Status of the resource.  Caller is responsible for checking
// for semantic differences before calling.
func (r *Reconciler) updateStatus(desired *v1alpha1.FooBinding) (*v1alpha1.FooBinding, error) {
	actual, err := r.Lister.FooBindings(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if reflect.DeepEqual(actual.Status, desired.Status) {
		return actual, nil
	}
	// Don't modify the informers copy
	existing := actual.DeepCopy()
	existing.Status = desired.Status
	return r.Client.BindingsV1alpha1().FooBindings(desired.Namespace).UpdateStatus(existing)
}
