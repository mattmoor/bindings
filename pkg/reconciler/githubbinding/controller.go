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

	fbclient "github.com/mattmoor/bindings/pkg/client/injection/client"
	fbinformer "github.com/mattmoor/bindings/pkg/client/injection/informers/bindings/v1alpha1/githubbinding"
	"github.com/mattmoor/bindings/pkg/reconciler"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"

	"github.com/mattmoor/bindings/pkg/apis/bindings/v1alpha1"
	"github.com/mattmoor/bindings/pkg/webhook/psbinding"
)

const (
	controllerAgentName = "githubbinding-controller"
)

// NewController returns a new HPA reconcile controller.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	fbInformer := fbinformer.Get(ctx)
	dc := dynamicclient.Get(ctx)

	// TODO(mattmoor): Share these across bindings?  It would miss the initial informer sync...
	psInformerFactory := &duck.TypedInformerFactory{
		Client:       dc,
		Type:         &v1alpha1.PodSpeccable{},
		ResyncPeriod: controller.GetResyncPeriod(ctx),
		StopChannel:  ctx.Done(),
	}

	c := &Reconciler{
		Base: reconciler.Base{
			GVR: v1alpha1.SchemeGroupVersion.WithResource("githubbindings"),
			Get: func(namespace string, name string) (*unstructured.Unstructured, error) {
				obj, err := fbInformer.Lister().GithubBindings(namespace).Get(name)
				if err != nil {
					return nil, err
				}
				return psbinding.ToUnstructured(obj)
			},
			DynamicClient: dc,
			Recorder: record.NewBroadcaster().NewRecorder(
				scheme.Scheme, corev1.EventSource{Component: controllerAgentName}),
		},
		Client: fbclient.Get(ctx),
		Lister: fbInformer.Lister(),
	}
	impl := controller.NewImpl(c, logger, "GithubBindings")

	logger.Info("Setting up event handlers")

	fbInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	c.Tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))
	c.Factory = &duck.CachedInformerFactory{
		Delegate: &duck.EnqueueInformerFactory{
			Delegate:     psInformerFactory,
			EventHandler: controller.HandleAll(c.Tracker.OnChanged),
		},
	}

	return impl
}

func ListAll(ctx context.Context, handler cache.ResourceEventHandler) psbinding.ListAll {
	fbInformer := fbinformer.Get(ctx)

	// Whenever a GithubBinding changes our webhook programming might change.
	fbInformer.Informer().AddEventHandler(handler)

	return func() ([]psbinding.Bindable, error) {
		l, err := fbInformer.Lister().List(labels.Everything())
		if err != nil {
			return nil, err
		}
		bl := make([]psbinding.Bindable, 0, len(l))
		for _, elt := range l {
			bl = append(bl, elt)
		}
		return bl, nil
	}

}
