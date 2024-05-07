package controllers

import (
	"context"

	"bitbucket.org/sudosweden/dockyards-backend/pkg/api/apiutil"
	dockyardsv1 "bitbucket.org/sudosweden/dockyards-backend/pkg/api/v1alpha2"
	"bitbucket.org/sudosweden/dockyards-git/pkg/repository"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// +kubebuilder:rbac:groups=dockyards.io,resources=containerimagedeployments,verbs=get;list;patch;watch
// +kubebuilder:rbac:groups=dockyards.io,resources=containerimagedeployments/status,verbs=patch
// +kubebuilder:rbac:groups=dockyards.io,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

type ContainerImageDeploymentReconciler struct {
	client.Client
	Repository *repository.GitRepository
}

func (r *ContainerImageDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	var containerImageDeployment dockyardsv1.ContainerImageDeployment
	err := r.Get(ctx, req.NamespacedName, &containerImageDeployment)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconcile container image deployment")

	if !containerImageDeployment.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &containerImageDeployment)
	}

	ownerDeployment, err := apiutil.GetOwnerDeployment(ctx, r.Client, &containerImageDeployment)
	if err != nil {
		logger.Error(err, "error getting owner deployment")

		return ctrl.Result{}, err
	}

	if ownerDeployment == nil {
		logger.Info("ignoring container image without owner deployment")

		return ctrl.Result{}, err
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

	var credential *corev1.Secret
	if containerImageDeployment.Spec.CredentialRef != nil {
		objectKey := client.ObjectKey{
			Name:      containerImageDeployment.Spec.CredentialRef.Name,
			Namespace: containerImageDeployment.Namespace,
		}

		err := r.Get(ctx, objectKey, credential)
		if err != nil {
			logger.Error(err, "error getting credential secret")

			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	repositoryURL, err := r.Repository.ReconcileContainerImageRepository(&containerImageDeployment, ownerDeployment, credential)
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

func (r *ContainerImageDeploymentReconciler) reconcileDelete(ctx context.Context, containerImageDeployment *dockyardsv1.ContainerImageDeployment) (ctrl.Result, error) {
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

func (r *ContainerImageDeploymentReconciler) SetupWithManager(ctx context.Context, m ctrl.Manager) error {
	scheme := m.GetScheme()

	_ = dockyardsv1.AddToScheme(scheme)

	err := ctrl.NewControllerManagedBy(m).For(&dockyardsv1.ContainerImageDeployment{}).Complete(r)
	if err != nil {
		return err
	}

	return nil
}
