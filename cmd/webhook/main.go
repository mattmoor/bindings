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

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics"

	"github.com/mattmoor/foo-binding/pkg/apis/bindings/v1alpha1"
	fbreconciler "github.com/mattmoor/foo-binding/pkg/reconciler/sinkbinding"
	fbwebhook "github.com/mattmoor/foo-binding/pkg/webhook/sinkbinding"
)

func NewResourceAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return resourcesemantics.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"webhook.bindings.mattmoor.dev",

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate and default.
		map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
			v1alpha1.SchemeGroupVersion.WithKind("SinkBinding"): &v1alpha1.SinkBinding{},
		},

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func main() {
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: "webhook",
		Port:        8443,
		SecretName:  "webhook-certs",
	})

	sharedmain.MainWithContext(ctx, "webhook",
		// Our singleton certificate controller.
		certificates.NewController,

		// Our singleton webhook admission controllers
		NewResourceAdmissionController,
		// TODO(mattmoor): Support config validation
		func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
			return fbwebhook.NewAdmissionController(ctx,
				// Name of the resource webhook.
				"sinkbindings.webhook.bindings.mattmoor.dev",

				// The path on which to serve the webhook.
				"/foo-binding",
			)
		},

		// Our actual controllers
		fbreconciler.NewController,
	)
}
