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

package slackbinding

import (
	"context"

	"github.com/mattmoor/bindings/pkg/apis/bindings/v1alpha1"
	clientset "github.com/mattmoor/bindings/pkg/client/clientset/versioned"
	listers "github.com/mattmoor/bindings/pkg/client/listers/bindings/v1alpha1"
	"github.com/mattmoor/bindings/pkg/webhook/psbinding"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// Reconciler implements controller.Reconciler for SlackBinding resources.
type Reconciler struct {
	psbinding.BaseReconciler

	// Client is used to write back status updates.
	Client clientset.Interface

	// Listers index properties about resources
	Lister listers.SlackBindingLister
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile implements controller.Reconciler
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the resource with this namespace/name.
	original, err := r.Lister.SlackBindings(namespace).Get(name)
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
	} else if err = r.UpdateStatus(ctx, resource); err != nil {
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

func (r *Reconciler) reconcile(ctx context.Context, fb *v1alpha1.SlackBinding) error {
	if fb.GetDeletionTimestamp() != nil {
		// Check for a DeletionTimestamp.  If present, elide the normal reconcile logic.
		// When a controller needs finalizer handling, it would go here.

		return r.reconcileDeletion(ctx, fb)
	}
	fb.Status.InitializeConditions()

	if err := r.EnsureFinalizer(ctx, fb); err != nil {
		return err
	}

	if err := r.ReconcileSubject(ctx, fb, fb.Do); err != nil {
		return err
	}

	fb.Status.ObservedGeneration = fb.Generation
	return nil
}

func (r *Reconciler) reconcileDeletion(ctx context.Context, fb *v1alpha1.SlackBinding) error {
	if !r.IsFinalizing(ctx, fb) {
		return nil
	}

	logging.FromContext(ctx).Infof("Removing the binding for %s", fb.Name)
	if err := r.ReconcileSubject(ctx, fb, fb.Undo); apierrs.IsNotFound(err) {
		// Everything is fine.
	} else if err != nil {
		return err
	}

	return r.RemoveFinalizer(ctx, fb)
}
