package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"net/http/cgi"
	"os"
	"os/signal"

	"bitbucket.org/sudosweden/dockyards-git/controller"
	"bitbucket.org/sudosweden/dockyards-git/pkg/repository"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	var gitProjectRoot string
	var gitCGIPath string
	var repositoryHostname string
	flag.StringVar(&gitProjectRoot, "git-project-root", "/tmp/dockyards-git", "git project root")
	flag.StringVar(&gitCGIPath, "git-cgi-path", "/usr/libexec/git-core/git-http-backend", "git cgi path")
	flag.StringVar(&repositoryHostname, "repository-hostname", "localhost:9002", "repository hostname")
	flag.Parse()

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

	m, err := manager.New(cfg, manager.Options{})
	if err != nil {
		logger.Error("error creating new manager", "err", err)

		os.Exit(1)
	}

	repository := repository.GitRepository{
		GitProjectRoot: gitProjectRoot,
		Logger:         logger,
		Hostname:       repositoryHostname,
	}

	err = (&controller.KustomizeDeploymentReconciler{
		Client:     m.GetClient(),
		Logger:     logger,
		Repository: &repository,
	}).SetupWithManager(ctx, m)
	if err != nil {
		logger.Error("error creating new kustomize controller", "err", err)

		os.Exit(1)
	}

	err = (&controller.ContainerImageDeploymentReconciler{
		Client:     m.GetClient(),
		Repository: &repository,
	}).SetupWithManager(ctx, m)
	if err != nil {
		logger.Error("error creating container image controller", "err", err)

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

	err = m.Start(ctx)
	if err != nil {
		logger.Error("error starting manager", "err", err)

		os.Exit(1)
	}
}
