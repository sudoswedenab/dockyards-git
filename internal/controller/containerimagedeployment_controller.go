package controller

import (
	"context"
	"errors"
	"log/slog"
	"net/url"

	"bitbucket.org/sudosweden/dockyards-backend/pkg/api/v1alpha1"
	"bitbucket.org/sudosweden/dockyards-git/pkg/repository"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// +kubebuilder:rbac:groups=dockyards.io,resources=containerimagedeployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=dockyards.io,resources=containerimagedeployments/status,verbs=patch

type ContainerImageDeploymentReconciler struct {
	client.Client

	Logger         *slog.Logger
	GitProjectRoot string
}

func (r *ContainerImageDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var containerImageDeployment v1alpha1.ContainerImageDeployment
	err := r.Get(ctx, req.NamespacedName, &containerImageDeployment)
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	r.Logger.Debug("reconcile container image deployment", "name", containerImageDeployment.Name)

	if !containerImageDeployment.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &containerImageDeployment)
	}

	if !controllerutil.ContainsFinalizer(&containerImageDeployment, finalizer) {
	}

	if containerImageDeployment.Status.RepositoryURL == "" {
		repoPath, err := repository.CreateContainerImageRepository(&containerImageDeployment, r.GitProjectRoot)
		if err != nil {
			return ctrl.Result{}, err
		}

		r.Logger.Debug("created repository for container image deployment", "path", repoPath)

		patch := client.MergeFrom(containerImageDeployment.DeepCopy())

		u := url.URL{
			Scheme: "http",
			Host:   "dockyards-git.dockyards",
			Path:   repoPath,
		}

		containerImageDeployment.Status.RepositoryURL = u.String()

		err = r.Status().Patch(ctx, &containerImageDeployment, patch)
		if err != nil {
			r.Logger.Error("error patching kustomized deployment", "err", err)

			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ContainerImageDeploymentReconciler) reconcileDelete(ctx context.Context, containerImageDeployment *v1alpha1.ContainerImageDeployment) (ctrl.Result, error) {
	return ctrl.Result{}, errors.New("not implemented")
}

func (r *ContainerImageDeploymentReconciler) SetupWithManager(ctx context.Context, manager ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(manager).For(&v1alpha1.ContainerImageDeployment{}).Complete(r)
	if err != nil {
		return err
	}

	return nil
}
