package awsclient

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

// PodIdentityAssociationClient defines the interface for AWS Pod Identity Association operations
type PodIdentityAssociationClient interface {
	// CreatePodIdentityAssociation creates a new Pod Identity Association
	CreatePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount, roleArn, assumeRoleArn string) error

	// UpdatePodIdentityAssociation updates an existing Pod Identity Association
	UpdatePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount, roleArn, assumeRoleArn string) error

	// DeletePodIdentityAssociation deletes a Pod Identity Association
	DeletePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount) error

	// GetPodIdentityAssociation retrieves a Pod Identity Association
	GetPodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount) (*PodIdentityAssociation, error)

	// AssociationExists checks if a Pod Identity Association exists for the given ServiceAccount
	AssociationExists(ctx context.Context, sa *corev1.ServiceAccount) (bool, error)

	// ListPodIdentityAssociations lists all Pod Identity Associations for the cluster
	ListPodIdentityAssociations(ctx context.Context) ([]*PodIdentityAssociation, error)
}

// PodIdentityAssociation represents a Pod Identity Association
type PodIdentityAssociation struct {
	ID                 string
	ClusterName        string
	Namespace          string
	ServiceAccountName string
	RoleArn            string
	AssumeRolePolicy   string
	Tags               map[string]string
	Status             string
	CreatedAt          *string
	ModifiedAt         *string
}

// AssociationStatus represents the status of a Pod Identity Association
type AssociationStatus string

const (
	AssociationStatusCreating AssociationStatus = "CREATING"
	AssociationStatusActive   AssociationStatus = "ACTIVE"
	AssociationStatusDeleting AssociationStatus = "DELETING"
	AssociationStatusFailed   AssociationStatus = "FAILED"
)
