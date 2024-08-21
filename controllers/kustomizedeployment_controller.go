package controllers

import (
	"context"

	dockyardsv1 "bitbucket.org/sudosweden/dockyards-backend/pkg/api/v1alpha2"
	"bitbucket.org/sudosweden/dockyards-git/pkg/repository"
	"github.com/fluxcd/pkg/runtime/patch"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// +kubebuilder:rbac:groups=dockyards.io,resources=kustomizedeployments,verbs=get;list;patch;watch
// +kubebuilder:rbac:groups=dockyards.io,resources=kustomizedeployments/status,verbs=patch

type KustomizeDeploymentReconciler struct {
	client.Client
	Repository *repository.GitRepository
}

func (r *KustomizeDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	logger := ctrl.LoggerFrom(ctx)

	var kustomizeDeployment dockyardsv1.KustomizeDeployment
	err := r.Get(ctx, req.NamespacedName, &kustomizeDeployment)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconcile kustomize deployment")

	patchHelper, err := patch.NewHelper(&kustomizeDeployment, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		err := patchHelper.Patch(ctx, &kustomizeDeployment)
		if err != nil {
			result = ctrl.Result{}
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	if !kustomizeDeployment.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &kustomizeDeployment)
	}

	if controllerutil.AddFinalizer(&kustomizeDeployment, finalizer) {
		return ctrl.Result{}, nil
	}

	repositoryURL, err := r.Repository.ReconcileKustomizeRepository(&kustomizeDeployment)
	if err != nil {
		logger.Error(err, "error reconciling repository")

		gitRepositoryReadyCondition := metav1.Condition{
			Type:    GitRepositoryReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  ReconciliationFailedReason,
			Message: err.Error(),
		}

		meta.SetStatusCondition(&kustomizeDeployment.Status.Conditions, gitRepositoryReadyCondition)

		return ctrl.Result{}, nil
	}

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

	return ctrl.Result{}, nil
}

func (r *KustomizeDeploymentReconciler) reconcileDelete(ctx context.Context, kustomizeDeployment *dockyardsv1.KustomizeDeployment) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	err := r.Repository.DeleteRepository(kustomizeDeployment)
	if err != nil {
		logger.Error(err, "error deleting repository")

		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(kustomizeDeployment, finalizer)

	logger.Info("deleted kustomize deployment")

	return ctrl.Result{}, nil
}

func (r *KustomizeDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	scheme := mgr.GetScheme()

	_ = dockyardsv1.AddToScheme(scheme)

	err := ctrl.NewControllerManagedBy(mgr).For(&dockyardsv1.KustomizeDeployment{}).Complete(r)
	if err != nil {
		return err
	}

	return nil
}
