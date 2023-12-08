package controller

import (
	"context"
	"errors"
	"log/slog"
	"net/url"

	"bitbucket.org/sudosweden/dockyards-backend/pkg/api/v1alpha1"
	"bitbucket.org/sudosweden/dockyards-git/pkg/repository"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// +kubebuilder:rbac:groups=dockyards.io,resources=kustomizedeployments,verbs=get;list;watch
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
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	logger.Debug("reconcile kustomize deployment")

	if !kustomizeDeployment.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &kustomizeDeployment)
	}

	if !controllerutil.ContainsFinalizer(&kustomizeDeployment, finalizer) {
	}

	repoPath, err := r.Repository.ReconcileKustomizeRepository(&kustomizeDeployment)
	if err != nil {
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
			r.Logger.Error("error patching kustomized deployment", "err", err)

			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	logger.Debug("reconciled repository for kustomized deployment", "path", repoPath)

	patch := client.MergeFrom(kustomizeDeployment.DeepCopy())

	u := url.URL{
		Scheme: "http",
		Host:   "dockyards-git.dockyards",
		Path:   repoPath,
	}

	if kustomizeDeployment.Status.RepositoryURL != u.String() {
		kustomizeDeployment.Status.RepositoryURL = u.String()
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
		r.Logger.Error("error patching kustomized deployment", "err", err)

		return ctrl.Result{}, err
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
