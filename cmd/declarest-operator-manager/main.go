// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	"github.com/crmarques/declarest/internal/operator/controllers"
	"github.com/crmarques/declarest/internal/operator/observability"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	var (
		metricsAddr           string
		probeAddr             string
		repositoryWebhookAddr string
		enableLeaderElection  bool
		enableWebhooks        bool
		watchNamespace        string
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&repositoryWebhookAddr, "repository-webhook-bind-address", ":8082", "The address the repository webhook endpoint binds to (set empty to disable).")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.BoolVar(&enableWebhooks, "enable-webhooks", true, "Enable admission webhooks for DeclaREST CRDs.")
	flag.StringVar(&watchNamespace, "watch-namespace", "", "Namespace to watch (empty means all namespaces).")
	zapOptions := zap.Options{Development: false}
	zapOptions.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOptions)))

	scheme := runtime.NewScheme()
	utilruntime.Must(k8sscheme.AddToScheme(scheme))
	utilruntime.Must(declarestv1alpha1.AddToScheme(scheme))

	ctx := ctrl.SetupSignalHandler()
	shutdownTelemetry, err := observability.Setup(ctx, "declarest-operator")
	if err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to initialize observability")
		os.Exit(1)
	}
	defer func() {
		if shutdownErr := shutdownTelemetry(ctx); shutdownErr != nil {
			ctrl.Log.WithName("shutdown").Error(shutdownErr, "failed to shutdown observability")
		}
	}()

	managerOptions := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "declarest-operator.declarest.io",
	}
	if namespace := strings.TrimSpace(watchNamespace); namespace != "" {
		managerOptions.Cache = cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				namespace: {},
			},
		}
	}

	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), managerOptions)
	if err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&controllers.ResourceRepositoryReconciler{
		Client:   manager.GetClient(),
		Scheme:   manager.GetScheme(),
		Recorder: manager.GetEventRecorderFor("resourcerepository-controller"),
	}).SetupWithManager(manager); err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to create ResourceRepository controller")
		os.Exit(1)
	}
	if err := (&controllers.ManagedServerReconciler{
		Client:   manager.GetClient(),
		Scheme:   manager.GetScheme(),
		Recorder: manager.GetEventRecorderFor("managedserver-controller"),
	}).SetupWithManager(manager); err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to create ManagedServer controller")
		os.Exit(1)
	}
	if err := (&controllers.SecretStoreReconciler{
		Client:   manager.GetClient(),
		Scheme:   manager.GetScheme(),
		Recorder: manager.GetEventRecorderFor("secretstore-controller"),
	}).SetupWithManager(manager); err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to create SecretStore controller")
		os.Exit(1)
	}
	if err := (&controllers.SyncPolicyReconciler{
		Client:   manager.GetClient(),
		Scheme:   manager.GetScheme(),
		Recorder: manager.GetEventRecorderFor("syncpolicy-controller"),
	}).SetupWithManager(manager); err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to create SyncPolicy controller")
		os.Exit(1)
	}
	if err := manager.Add(&controllers.RepositoryWebhookServer{
		Client:         manager.GetClient(),
		Recorder:       manager.GetEventRecorderFor("repository-webhook"),
		BindAddress:    repositoryWebhookAddr,
		WatchNamespace: watchNamespace,
	}); err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to start repository webhook server")
		os.Exit(1)
	}
	if enableWebhooks {
		if err := (&declarestv1alpha1.ResourceRepository{}).SetupWebhookWithManager(manager); err != nil {
			ctrl.Log.WithName("setup").Error(err, "unable to create ResourceRepository webhook")
			os.Exit(1)
		}
		if err := (&declarestv1alpha1.ManagedServer{}).SetupWebhookWithManager(manager); err != nil {
			ctrl.Log.WithName("setup").Error(err, "unable to create ManagedServer webhook")
			os.Exit(1)
		}
		if err := (&declarestv1alpha1.SecretStore{}).SetupWebhookWithManager(manager); err != nil {
			ctrl.Log.WithName("setup").Error(err, "unable to create SecretStore webhook")
			os.Exit(1)
		}
		if err := (&declarestv1alpha1.SyncPolicy{}).SetupWebhookWithManager(manager); err != nil {
			ctrl.Log.WithName("setup").Error(err, "unable to create SyncPolicy webhook")
			os.Exit(1)
		}
	}

	if err := manager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := manager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	if err := manager.AddReadyzCheck("informers", func(_ *http.Request) error {
		if !manager.GetCache().WaitForCacheSync(ctx) {
			return fmt.Errorf("informer caches not synced")
		}
		return nil
	}); err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to set up informer sync check")
		os.Exit(1)
	}

	if err := manager.Start(ctx); err != nil {
		ctrl.Log.WithName("setup").Error(err, "problem running manager")
		os.Exit(1)
	}
}
