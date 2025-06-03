// Copyright 2025 Sudo Sweden AB
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
	"context"
	"log/slog"
	"net/http"
	"net/http/cgi"
	"os"
	"os/signal"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"github.com/sudoswedenab/dockyards-git/controllers"
	"github.com/sudoswedenab/dockyards-git/pkg/repository"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	var gitProjectRoot string
	var gitCGIPath string
	var repositoryHostname string
	var metricsBindAddress string
	pflag.StringVar(&gitProjectRoot, "git-project-root", "/tmp/dockyards-git", "git project root")
	pflag.StringVar(&gitCGIPath, "git-cgi-path", "/usr/libexec/git-core/git-http-backend", "git cgi path")
	pflag.StringVar(&repositoryHostname, "repository-hostname", "localhost:9002", "repository hostname")
	pflag.StringVar(&metricsBindAddress, "metrics-bind-address", "0", "metricsx bind address")
	pflag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slogr := logr.FromSlogHandler(logger.Handler())

	ctrl.SetLogger(slogr)

	cfg, err := config.GetConfig()
	if err != nil {
		logger.Error("error getting config", "err", err)

		os.Exit(1)
	}

	options := manager.Options{
		Metrics: metricsserver.Options{
			BindAddress: metricsBindAddress,
		},
	}

	mgr, err := manager.New(cfg, options)
	if err != nil {
		logger.Error("error creating new manager", "err", err)

		os.Exit(1)
	}

	repository := repository.GitRepository{
		GitProjectRoot: gitProjectRoot,
		Logger:         logger,
		Hostname:       repositoryHostname,
	}

	err = (&controllers.DockyardsWorktreeReconciler{
		Client:     mgr.GetClient(),
		Repository: &repository,
	}).SetupWithManager(mgr)
	if err != nil {
		logger.Error("error creating dockyards worktree controller", "err", err)

		os.Exit(1)
	}

	cgiHandler := cgi.Handler{
		Path: gitCGIPath,
		Dir:  gitProjectRoot,
		Env: []string{
			"GIT_PROJECT_ROOT=" + gitProjectRoot,
			"GIT_HTTP_EXPORT_ALL=true",
		},
	}

	httpServer := http.Server{
		Handler: &cgiHandler,
		Addr:    ":9002",
	}

	go func() {
		err := httpServer.ListenAndServe()
		if err != nil {
			logger.Error("error serving http", "err", err)

			os.Exit(1)
		}
	}()

	err = mgr.Start(ctx)
	if err != nil {
		logger.Error("error starting manager", "err", err)

		os.Exit(1)
	}
}
