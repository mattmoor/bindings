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

package githubbinding

import (
	"context"

	// Injection stuff
	fbinformer "github.com/mattmoor/bindings/pkg/client/injection/informers/bindings/v1alpha1/githubbinding"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	mwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1beta1/mutatingwebhookconfiguration"
	secretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
)

// NewAdmissionController constructs a reconciler
func NewAdmissionController(
	ctx context.Context,
	name, path string,
) *controller.Impl {

	client := kubeclient.Get(ctx)
	mwhInformer := mwhinformer.Get(ctx)
	secretInformer := secretinformer.Get(ctx)
	fbInformer := fbinformer.Get(ctx)
	options := webhook.GetOptions(ctx)

	wh := &reconciler{
		name: name,
		path: path,

		secretName: options.SecretName,

		client:       client,
		mwhlister:    mwhInformer.Lister(),
		secretlister: secretInformer.Lister(),
		fblister:     fbInformer.Lister(),
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

	// Whenever a GithubBinding changes our webhook programming might change.
	fbInformer.Informer().AddEventHandler(handler)

	return c
}
