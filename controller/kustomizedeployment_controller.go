package controller

import (
	"context"
	"log/slog"

	"bitbucket.org/sudosweden/dockyards-backend/pkg/api/v1alpha1"
	"bitbucket.org/sudosweden/dockyards-git/pkg/repository"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// +kubebuilder:rbac:groups=dockyards.io,resources=kustomizedeployments,verbs=get;list;patch;watch
// +kubebuilder:rbac:groups=dockyards.io,resources=kustomizedeployments/status,verbs=patch

type KustomizeDeploymentReconciler struct {
	client.Client
	Logger     *slog.Logger
	Repository *repository.GitRepository
}

func (r *KustomizeDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.With("name", req.Name, "namespace", req.Namespace)

	var kustomizeDeployment v1alpha1.KustomizeDeployment
	err := r.Get(ctx, req.NamespacedName, &kustomizeDeployment)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Debug("reconcile kustomize deployment")

	if !kustomizeDeployment.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &kustomizeDeployment)
	}

	if !controllerutil.ContainsFinalizer(&kustomizeDeployment, finalizer) {
		logger.Debug("reconcile missing finalizer", "finalizer", finalizer)

		patch := client.MergeFrom(kustomizeDeployment.DeepCopy())

		controllerutil.AddFinalizer(&kustomizeDeployment, finalizer)

		err := r.Patch(ctx, &kustomizeDeployment, patch)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	repositoryURL, err := r.Repository.ReconcileKustomizeRepository(&kustomizeDeployment)
	if err != nil {
		logger.Error("error reconciling repository for kustomize deployment", "err", err)

		gitRepositoryReadyCondition := metav1.Condition{
			Type:    GitRepositoryReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  ReconciliationFailedReason,
			Message: err.Error(),
		}

		patch := client.MergeFrom(kustomizeDeployment.DeepCopy())

		meta.SetStatusCondition(&kustomizeDeployment.Status.Conditions, gitRepositoryReadyCondition)

		err = r.Status().Patch(ctx, &kustomizeDeployment, patch)
		if err != nil {
			logger.Error("error patching kustomize deployment", "err", err)

			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	logger.Debug("reconciled repository for kustomize deployment")

	patch := client.MergeFrom(kustomizeDeployment.DeepCopy())

	if kustomizeDeployment.Status.RepositoryURL != repositoryURL {
		kustomizeDeployment.Status.RepositoryURL = repositoryURL
	}

	gitRepositoryReadyCondition := metav1.Condition{
		Type:               GitRepositoryReadyCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: kustomizeDeployment.Generation,
		Reason:             ReconciliationSucceededReason,
	}

	meta.SetStatusCondition(&kustomizeDeployment.Status.Conditions, gitRepositoryReadyCondition)

	err = r.Status().Patch(ctx, &kustomizeDeployment, patch)
	if err != nil {
		r.Logger.Error("error patching kustomize deployment", "err", err)

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KustomizeDeploymentReconciler) reconcileDelete(ctx context.Context, kustomizeDeployment *v1alpha1.KustomizeDeployment) (ctrl.Result, error) {
	logger := r.Logger.With("name", kustomizeDeployment.Name, "namespace", kustomizeDeployment.Namespace)

	err := r.Repository.DeleteRepository(kustomizeDeployment)
	if err != nil {
		logger.Error("error deleting repository", "err", err)

		return ctrl.Result{}, err
	}

	logger.Debug("deleted repository for kustomize deployment")

	patch := client.MergeFrom(kustomizeDeployment.DeepCopy())

	controllerutil.RemoveFinalizer(kustomizeDeployment, finalizer)

	err = r.Patch(ctx, kustomizeDeployment, patch)
	if err != nil {
		logger.Error("error patching kustomize deployment", "err", err)

		return ctrl.Result{}, err
	}

	logger.Debug("deleted kustomize deployment")

	return ctrl.Result{}, nil
}

func (r *KustomizeDeploymentReconciler) SetupWithManager(ctx context.Context, manager ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(manager).For(&v1alpha1.KustomizeDeployment{}).Complete(r)
	if err != nil {
		return err
	}

	return nil
}
