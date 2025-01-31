package controllers

const (
	RepositoryReadyCondition = "RepositoryReady"

	RepositoryReconciledReason       = "RepositoryReconciled"
	ReconcileRepositoryErrorReason   = "ReconcileRepositoryError"
	DeleteRepositoryErrorReason      = "DeleteRepositoryError"
	WaitingForOwnerDeploymentReason  = "WaitingForOwnerDeployment"
	InvalidCredentialReferenceReason = "InvalidCredentialReference"
)
