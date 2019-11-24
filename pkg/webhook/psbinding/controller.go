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

	// Injection stuff

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	mwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1beta1/mutatingwebhookconfiguration"
	secretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"
	"knative.dev/pkg/webhook"
)

// Bindable is implemented by Bindings whose subjects as duckv1.WithPod.
type Bindable interface {
	kmeta.Accessor
	kmeta.OwnerRefable

	GetSubject() tracker.Reference

	MarkBindingAvailable()
	MarkBindingUnavailable(reason string, message string)

	Do(context.Context, *duckv1.WithPod)
	Undo(context.Context, *duckv1.WithPod)
}

type ListAll func() ([]Bindable, error)

type GetListAll func(context.Context, cache.ResourceEventHandler) ListAll

type BindableContext func(context.Context, Bindable) context.Context

// NewAdmissionController constructs a reconciler
func NewAdmissionController(
	ctx context.Context,
	name, path string,
	la GetListAll,
	WithContext BindableContext,
) *controller.Impl {

	client := kubeclient.Get(ctx)
	mwhInformer := mwhinformer.Get(ctx)
	secretInformer := secretinformer.Get(ctx)
	options := webhook.GetOptions(ctx)

	wh := &reconciler{
		name: name,
		path: path,

		secretName: options.SecretName,

		WithContext: WithContext,

		client:       client,
		mwhlister:    mwhInformer.Lister(),
		secretlister: secretInformer.Lister(),
	}

	logger := logging.FromContext(ctx)
	c := controller.NewImpl(wh, logger, "GithubBindingWebhook")

	// It doesn't matter what we enqueue because we will always Reconcile
	// the named MWH resource.
	handler := controller.HandleAll(c.EnqueueSentinel(types.NamespacedName{}))

	// Reconcile when the named MutatingWebhookConfiguration changes.
	mwhInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithName(name),
		Handler:    handler,
	})

	// Reconcile when the cert bundle changes.
	secretInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(system.Namespace(), wh.secretName),
		Handler:    handler,
	})

	// Give the reconciler a way to list all of the Bindable resources,
	// and configure the controller to handle changes to those resources.
	wh.listall = la(ctx, handler)

	return c
}
