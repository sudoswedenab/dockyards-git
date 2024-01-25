package controller

import (
	"context"

	"bitbucket.org/sudosweden/dockyards-backend/pkg/api/v1alpha1"
	"bitbucket.org/sudosweden/dockyards-git/pkg/repository"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// +kubebuilder:rbac:groups=dockyards.io,resources=containerimagedeployments,verbs=get;list;patch;watch
// +kubebuilder:rbac:groups=dockyards.io,resources=containerimagedeployments/status,verbs=patch

type ContainerImageDeploymentReconciler struct {
	client.Client
	Repository *repository.GitRepository
}

func (r *ContainerImageDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	var containerImageDeployment v1alpha1.ContainerImageDeployment
	err := r.Get(ctx, req.NamespacedName, &containerImageDeployment)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconcile container image deployment")

	if !containerImageDeployment.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &containerImageDeployment)
	}

	if !controllerutil.ContainsFinalizer(&containerImageDeployment, finalizer) {
		patch := client.MergeFrom(containerImageDeployment.DeepCopy())

		controllerutil.AddFinalizer(&containerImageDeployment, finalizer)

		err := r.Patch(ctx, &containerImageDeployment, patch)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	repositoryURL, err := r.Repository.ReconcileContainerImageRepository(&containerImageDeployment)
	if err != nil {
		logger.Error(err, "error reconciling repository for container image deployment")

		gitRepositoryReadyCondition := metav1.Condition{
			Type:    GitRepositoryReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  ReconciliationFailedReason,
			Message: err.Error(),
		}

		patch := client.MergeFrom(containerImageDeployment.DeepCopy())

		meta.SetStatusCondition(&containerImageDeployment.Status.Conditions, gitRepositoryReadyCondition)

		err = r.Status().Patch(ctx, &containerImageDeployment, patch)
		if err != nil {
			logger.Error(err, "error patching status")

			return ctrl.Result{}, err
		}

		return ctrl.Result{}, err
	}

	logger.Info("reconciled repository for container image deployment")

	patch := client.MergeFrom(containerImageDeployment.DeepCopy())

	if containerImageDeployment.Status.RepositoryURL != repositoryURL {
		containerImageDeployment.Status.RepositoryURL = repositoryURL
	}

	gitRepositoryReadyCondition := metav1.Condition{
		Type:               GitRepositoryReadyCondition,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: containerImageDeployment.Generation,
		Reason:             ReconciliationSucceededReason,
	}

	meta.SetStatusCondition(&containerImageDeployment.Status.Conditions, gitRepositoryReadyCondition)

	err = r.Status().Patch(ctx, &containerImageDeployment, patch)
	if err != nil {
		logger.Error(err, "error patching container image deployment")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ContainerImageDeploymentReconciler) reconcileDelete(ctx context.Context, containerImageDeployment *v1alpha1.ContainerImageDeployment) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	err := r.Repository.DeleteRepository(containerImageDeployment)
	if err != nil {
		logger.Error(err, "error deleting repository")

		return ctrl.Result{}, err
	}

	logger.Info("deleted repository for container image deployment")

	patch := client.MergeFrom(containerImageDeployment.DeepCopy())

	controllerutil.RemoveFinalizer(containerImageDeployment, finalizer)

	err = r.Patch(ctx, containerImageDeployment, patch)
	if err != nil {
		logger.Error(err, "error patching container image deployment")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ContainerImageDeploymentReconciler) SetupWithManager(ctx context.Context, manager ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(manager).For(&v1alpha1.ContainerImageDeployment{}).Complete(r)
	if err != nil {
		return err
	}

	return nil
}
