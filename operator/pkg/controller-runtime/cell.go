// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package controllerruntime

import (
	"context"
	"fmt"

	"github.com/bombsimon/logrusr/v4"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlRuntime "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/cilium/cilium/pkg/hive"
	"github.com/cilium/cilium/pkg/hive/cell"
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
)

// Cell integrates the components of the controller-runtime library into Hive.
// The Kubernetes controller-runtime Project is a set of go libraries for building Controllers.
// See https://github.com/kubernetes-sigs/controller-runtime for further information.
var Cell = cell.Module(
	"controller-runtime",
	"Manages the controller-runtime integration and its components",

	cell.Provide(newScheme),
	cell.Provide(newManager),
)

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ciliumv2.AddToScheme(scheme))

	return scheme
}

func newManager(lc hive.Lifecycle, logger logrus.FieldLogger, scheme *runtime.Scheme) (ctrlRuntime.Manager, error) {
	ctrlRuntime.SetLogger(logrusr.New(logger))

	mgr, err := ctrlRuntime.NewManager(ctrlRuntime.GetConfigOrDie(), ctrlRuntime.Options{
		Scheme: scheme,
		// Disable controller metrics server in favour of cilium's metrics server.
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		Logger: logrusr.New(logger),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create new controller-runtime manager: %w", err)
	}

	ctx, ctxCancel := context.WithCancel(context.Background())

	lc.Append(hive.Hook{
		OnStart: func(_ hive.HookContext) error {
			go func() {
				if err := mgr.Start(ctx); err != nil {
					logger.WithError(err).Error("Unable to start manager")
				}
			}()
			return nil
		},
		OnStop: func(_ hive.HookContext) error {
			ctxCancel()
			return nil
		},
	})

	return mgr, nil
}
