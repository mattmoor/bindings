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

package sinkbinding

import (
	"context"

	fbclient "github.com/mattmoor/foo-binding/pkg/client/injection/client"
	fbinformer "github.com/mattmoor/foo-binding/pkg/client/injection/informers/bindings/v1alpha1/sinkbinding"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"

	"github.com/mattmoor/foo-binding/pkg/apis/bindings/v1alpha1"
)

const (
	controllerAgentName = "sinkbinding-controller"
)

// NewController returns a new HPA reconcile controller.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	fbInformer := fbinformer.Get(ctx)
	dc := dynamicclient.Get(ctx)

	psInformerFactory := &duck.TypedInformerFactory{
		Client:       dc,
		Type:         &v1alpha1.PodSpeccable{},
		ResyncPeriod: controller.GetResyncPeriod(ctx),
		StopChannel:  ctx.Done(),
	}

	c := &Reconciler{
		Client:        fbclient.Get(ctx),
		DynamicClient: dc,
		Lister:        fbInformer.Lister(),
		Recorder: record.NewBroadcaster().NewRecorder(
			scheme.Scheme, corev1.EventSource{Component: controllerAgentName}),
	}
	impl := controller.NewImpl(c, logger, "SinkBindings")

	logger.Info("Setting up event handlers")

	fbInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	c.Resolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	c.Tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))
	c.Factory = &duck.CachedInformerFactory{
		Delegate: &duck.EnqueueInformerFactory{
			Delegate:     psInformerFactory,
			EventHandler: controller.HandleAll(c.Tracker.OnChanged),
		},
	}

	return impl
}
