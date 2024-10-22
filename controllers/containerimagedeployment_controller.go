package controllers

import (
	"context"

	"bitbucket.org/sudosweden/dockyards-backend/pkg/api/apiutil"
	dockyardsv1 "bitbucket.org/sudosweden/dockyards-backend/pkg/api/v1alpha3"
	"bitbucket.org/sudosweden/dockyards-git/pkg/repository"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
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

func (r *ContainerImageDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	logger := ctrl.LoggerFrom(ctx)

	var containerImageDeployment dockyardsv1.ContainerImageDeployment
	err := r.Get(ctx, req.NamespacedName, &containerImageDeployment)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconcile container image deployment")

	patchHelper, err := patch.NewHelper(&containerImageDeployment, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		err := patchHelper.Patch(ctx, &containerImageDeployment)
		if err != nil {
			result = ctrl.Result{}
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	if !containerImageDeployment.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &containerImageDeployment)
	}

	ownerDeployment, err := apiutil.GetOwnerDeployment(ctx, r.Client, &containerImageDeployment)
	if err != nil {
		return ctrl.Result{}, err
	}

	if ownerDeployment == nil {
		conditions.MarkFalse(&containerImageDeployment, RepositoryReadyCondition, WaitingForOwnerDeploymentReason, "")

		return ctrl.Result{}, nil
	}

	if controllerutil.AddFinalizer(&containerImageDeployment, finalizer) {
		return ctrl.Result{}, nil
	}

	var credential *corev1.Secret
	if containerImageDeployment.Spec.CredentialRef != nil {
		objectKey := client.ObjectKey{
			Name:      containerImageDeployment.Spec.CredentialRef.Name,
			Namespace: containerImageDeployment.Namespace,
		}

		var secret corev1.Secret
		err := r.Get(ctx, objectKey, &secret)
		if err != nil {
			conditions.MarkFalse(&containerImageDeployment, RepositoryReadyCondition, InvalidCredentialReferenceReason, "%s", err)

			return ctrl.Result{}, nil
		}

		credential = &secret
	}

	repositoryURL, err := r.Repository.ReconcileContainerImageRepository(&containerImageDeployment, ownerDeployment, credential)
	if err != nil {
		conditions.MarkFalse(&containerImageDeployment, RepositoryReadyCondition, ReconcileRepositoryErrorReason, "%s", err)

		return ctrl.Result{}, err
	}

	logger.Info("reconciled repository for container image deployment")

	if containerImageDeployment.Status.RepositoryURL != repositoryURL {
		containerImageDeployment.Status.RepositoryURL = repositoryURL
	}

	conditions.MarkTrue(&containerImageDeployment, RepositoryReadyCondition, RepositoryReconciledReason, "")

	return ctrl.Result{}, nil
}

func (r *ContainerImageDeploymentReconciler) reconcileDelete(ctx context.Context, containerImageDeployment *dockyardsv1.ContainerImageDeployment) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	err := r.Repository.DeleteRepository(containerImageDeployment)
	if err != nil {
		conditions.MarkFalse(containerImageDeployment, RepositoryReadyCondition, DeleteRepositoryErrorReason, "%s", err)

		return ctrl.Result{}, nil
	}

	logger.Info("deleted repository for container image deployment")

	controllerutil.RemoveFinalizer(containerImageDeployment, finalizer)

	return ctrl.Result{}, nil
}

func (r *ContainerImageDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	scheme := mgr.GetScheme()

	_ = dockyardsv1.AddToScheme(scheme)

	err := ctrl.NewControllerManagedBy(mgr).For(&dockyardsv1.ContainerImageDeployment{}).Complete(r)
	if err != nil {
		return err
	}

	return nil
}
