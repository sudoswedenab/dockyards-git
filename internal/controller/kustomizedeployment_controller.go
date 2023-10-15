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

// +kubebuilder:rbac:groups=dockyards.io,resources=kustomizedeployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=dockyards.io,resources=kustomizedeployments/status,verbs=patch

type KustomizeDeploymentReconciler struct {
	client.Client

	Logger         *slog.Logger
	GitProjectRoot string
}

func (r *KustomizeDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var kustomizeDeployment v1alpha1.KustomizeDeployment
	err := r.Get(ctx, req.NamespacedName, &kustomizeDeployment)
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	r.Logger.Debug("reconcile kustomize deployment", "name", kustomizeDeployment.Name)

	if !kustomizeDeployment.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &kustomizeDeployment)
	}

	if !controllerutil.ContainsFinalizer(&kustomizeDeployment, finalizer) {
	}

	if kustomizeDeployment.Status.RepositoryURL == "" {
		repoPath, err := repository.CreateKustomizeRepository(&kustomizeDeployment, r.GitProjectRoot)
		if err != nil {
			return ctrl.Result{}, err
		}

		r.Logger.Debug("created repository for kustomized deployment", "path", repoPath)

		patch := client.MergeFrom(kustomizeDeployment.DeepCopy())

		u := url.URL{
			Scheme: "http",
			Host:   "dockyards-git.dockyards",
			Path:   repoPath + ".git",
		}

		kustomizeDeployment.Status.RepositoryURL = u.String()

		err = r.Status().Patch(ctx, &kustomizeDeployment, patch)
		if err != nil {
			r.Logger.Error("error patching kustomized deployment", "err", err)

			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *KustomizeDeploymentReconciler) reconcileDelete(ctx context.Context, kustomizeDeployment *v1alpha1.KustomizeDeployment) (ctrl.Result, error) {
	return ctrl.Result{}, errors.New("not implemented")
}

func (r *KustomizeDeploymentReconciler) SetupWithManager(ctx context.Context, manager ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(manager).For(&v1alpha1.KustomizeDeployment{}).Complete(r)
	if err != nil {
		return err
	}

	return nil
}
