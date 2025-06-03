package controllers

import (
	"context"

	"bitbucket.org/sudosweden/dockyards-git/pkg/repository"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	dockyardsv1 "github.com/sudoswedenab/dockyards-backend/api/v1alpha3"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=dockyards.io,resources=worktrees/status,verbs=patch
// +kubebuilder:rbac:groups=dockyards.io,resources=worktrees,verbs=get;list;watch

type DockyardsWorktreeReconciler struct {
	client.Client
	Repository repository.Repository
}

func (r *DockyardsWorktreeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	logger := ctrl.LoggerFrom(ctx)

	var dockyardsWorktree dockyardsv1.Worktree
	err := r.Get(ctx, req.NamespacedName, &dockyardsWorktree)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconcile")

	patchHelper, err := patch.NewHelper(&dockyardsWorktree, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		err := patchHelper.Patch(ctx, &dockyardsWorktree)
		if err != nil {
			result = ctrl.Result{}
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	reference, url, err := r.Repository.ReconcileWorktree(&dockyardsWorktree)
	if err != nil {
		conditions.MarkFalse(&dockyardsWorktree, RepositoryReadyCondition, ReconcileRepositoryErrorReason, "%s", err)

		return ctrl.Result{}, nil
	}

	dockyardsWorktree.Status.CommitHash = ptr.To(reference.Hash().String())
	dockyardsWorktree.Status.ReferenceName = ptr.To(reference.Name().String())
	dockyardsWorktree.Status.URL = ptr.To(url.String())

	conditions.MarkTrue(&dockyardsWorktree, RepositoryReadyCondition, dockyardsv1.ReadyReason, "")

	return ctrl.Result{}, nil
}

func (r *DockyardsWorktreeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	scheme := mgr.GetScheme()

	_ = dockyardsv1.AddToScheme(scheme)

	err := ctrl.NewControllerManagedBy(mgr).For(&dockyardsv1.Worktree{}).Complete(r)
	if err != nil {
		return err
	}

	return nil
}
