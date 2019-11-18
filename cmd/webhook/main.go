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

package main

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics"

	"github.com/mattmoor/bindings/pkg/apis/bindings/v1alpha1"
	"github.com/mattmoor/bindings/pkg/reconciler/githubbinding"
	"github.com/mattmoor/bindings/pkg/reconciler/slackbinding"
	"github.com/mattmoor/bindings/pkg/reconciler/twitterbinding"
	"github.com/mattmoor/bindings/pkg/webhook/psbinding"
)

func NewResourceAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return resourcesemantics.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"webhook.bindings.mattmoor.dev",

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate and default.
		map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
			v1alpha1.SchemeGroupVersion.WithKind("GithubBinding"):  &v1alpha1.GithubBinding{},
			v1alpha1.SchemeGroupVersion.WithKind("SlackBinding"):   &v1alpha1.SlackBinding{},
			v1alpha1.SchemeGroupVersion.WithKind("TwitterBinding"): &v1alpha1.TwitterBinding{},
		},

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func NewBindingWebhook(resource string, gla psbinding.GetListAll, wc psbinding.BindableContext) injection.ControllerConstructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return psbinding.NewAdmissionController(ctx,
			// Name of the resource webhook.
			fmt.Sprintf("%s.webhook.bindings.mattmoor.dev", resource),

			// The path on which to serve the webhook.
			fmt.Sprintf("/%s", resource),

			// How to get all the Bindables for configuring the mutating webhook.
			gla,

			// How to setup the context prior to invoking Do/Undo.
			wc,
		)
	}
}

func main() {
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: "webhook",
		Port:        8443,
		SecretName:  "webhook-certs",
	})

	nop := func(ctx context.Context, b psbinding.Bindable) context.Context {
		return ctx
	}

	sharedmain.MainWithContext(ctx, "webhook",
		// Our singleton certificate controller.
		certificates.NewController,

		// Our singleton webhook admission controllers
		NewResourceAdmissionController,
		// TODO(mattmoor): Pull in the resource semantics split.
		// TODO(mattmoor): Support config validation

		// For each binding we have a controller and a binding webhook.
		githubbinding.NewController, NewBindingWebhook("githubbindings", githubbinding.ListAll, nop),
		slackbinding.NewController, NewBindingWebhook("slackbindings", slackbinding.ListAll, nop),
		twitterbinding.NewController, NewBindingWebhook("twitterbindings", twitterbinding.ListAll, nop),
	)
}
