// Package awsclient provides AWS EKS Pod Identity Association management functionality.
//
// This package implements an interface for managing AWS EKS Pod Identity Associations,
// which allow Kubernetes ServiceAccounts to assume AWS IAM roles without requiring
// long-lived credentials. The package provides CRUD operations for Pod Identity
// Associations and integrates with the AWS EKS service using the AWS SDK for Go v2.
//
// Key features:
//   - Create, update, delete, and list Pod Identity Associations
//   - Support for both base roles and assume role scenarios
//   - Automatic association tracking via ServiceAccount annotations
//   - Idempotent operations with client request tokens
//   - Comprehensive error handling and logging
//
// The main interface AWSClient provides all necessary operations for managing
// Pod Identity Associations in an EKS cluster, with the Client struct providing
// the concrete implementation using the AWS SDK.
package awsclient

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

// AWSClient interface for Pod Identity operations (consolidated)
type AWSClient interface {
	CreatePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount, roleArn, assumeRoleArn string) (string, error)
	UpdatePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount, roleArn, assumeRoleArn string) (string, error)
	DeletePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount) error
	AssociationExists(ctx context.Context, sa *corev1.ServiceAccount) (bool, error)
	GetPodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount) (*PodIdentityAssociation, error)
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
