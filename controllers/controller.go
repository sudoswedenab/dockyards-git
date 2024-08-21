package controllers

const (
	finalizer = "dockyards.io/git-controller"
)

const (
	RepositoryReadyCondition = "RepositoryReady"

	RepositoryReconciledReason       = "RepositoryReconciled"
	ReconcileRepositoryErrorReason   = "ReconcileRepositoryError"
	DeleteRepositoryErrorReason      = "DeleteRepositoryError"
	WaitingForOwnerDeploymentReason  = "WaitingForOwnerDeployment"
	InvalidCredentialReferenceReason = "InvalidCredentialReference"
)
