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
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"

	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/version"
	"knative.dev/pkg/webhook"

	"github.com/mattmoor/foo-binding/pkg/apis/bindings/v1alpha1"
)

const (
	component = "webhook"
)

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

// type Store interface {
// 	WatchConfigs(configmap.Watcher)
// 	ToContext(context.Context) context.Context
// }

// type StoreFactory func(*zap.SugaredLogger) Store

func main() {
	flag.Parse()

	// Set up signals so we handle the first shutdown signal gracefully.
	ctx := signals.NewContext()

	// Report stats on Go memory usage every 30 seconds.
	msp := metrics.NewMemStatsAll()
	msp.Start(ctx, 30*time.Second)
	if err := view.Register(msp.DefaultViews()...); err != nil {
		log.Fatalf("Error exporting go memstats view: %v", err)
	}

	cfg, err := sharedmain.GetConfig(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatal("Failed to get cluster config:", err)
	}

	log.Printf("Registering %d clients", len(injection.Default.GetClients()))
	log.Printf("Registering %d informer factories", len(injection.Default.GetInformerFactories()))
	log.Printf("Registering %d informers", len(injection.Default.GetInformers()))

	ctx, _ = injection.Default.SetupInformers(ctx, cfg)
	kubeClient := kubeclient.Get(ctx)

	config, err := sharedmain.GetLoggingConfig(ctx)
	if err != nil {
		log.Fatal("Error loading/parsing logging configuration:", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(config, component)
	defer logger.Sync()
	logger = logger.With(zap.String(logkey.ControllerType, component))

	if err := version.CheckMinimumVersion(kubeClient.Discovery()); err != nil {
		logger.Fatalw("Version check failed", err)
	}

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace())
	configMapWatcher.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, component))

	// // If you want to control Defaulting or Validation, you can attach config state
	// // to the context by watching the configmap here, and then uncommenting the logic
	// // below.
	// stores := make([]Store, 0, len(factories))
	// for _, sf := range factories {
	// 	store := sf(logger)
	// 	store.WatchConfigs(configMapWatcher)
	// 	stores = append(stores, store)
	// }

	if err = configMapWatcher.Start(ctx.Done()); err != nil {
		logger.Fatalw("Failed to start the ConfigMap watcher", zap.Error(err))
	}

	options := webhook.ControllerOptions{
		ServiceName:                     "webhook",
		DeploymentName:                  "webhook",
		Namespace:                       system.Namespace(),
		Port:                            8443,
		SecretName:                      "webhook-certs",
		ResourceMutatingWebhookName:     fmt.Sprintf("webhook.%s.knative.dev", system.Namespace()),
		ResourceAdmissionControllerPath: "/",
	}

	resourceHandlers := map[schema.GroupVersionKind]webhook.GenericCRD{
		v1alpha1.SchemeGroupVersion.WithKind("FooBinding"): &v1alpha1.FooBinding{},
	}

	resourceAdmissionController := webhook.NewResourceAdmissionController(resourceHandlers, options, true)
	admissionControllers := map[string]webhook.AdmissionController{
		options.ResourceAdmissionControllerPath: resourceAdmissionController,
	}

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		return ctx
	}

	controller, err := webhook.New(kubeClient, options, admissionControllers, logger, ctxFunc)

	if err != nil {
		logger.Fatalw("Failed to create admission controller", zap.Error(err))
	}

	profilingHandler := profiling.NewHandler(logger, false)
	profilingServer := profiling.NewServer(profilingHandler)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return controller.Run(ctx.Done())
	})
	eg.Go(profilingServer.ListenAndServe)

	// This will block until either a signal arrives or one of the grouped functions
	// returns an error.
	<-egCtx.Done()

	profilingServer.Shutdown(context.Background())
	// Don't forward ErrServerClosed as that indicates we're already shutting down.
	if err := eg.Wait(); err != nil && err != http.ErrServerClosed {
		logger.Errorw("Error while running server", zap.Error(err))
	}
}
