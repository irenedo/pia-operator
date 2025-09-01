// Package k8sclient provides a simplified interface for Kubernetes client operations.
//
// This package wraps the controller-runtime client to provide focused operations
// for ServiceAccount resources. It abstracts common Kubernetes API operations
// with a clean interface that's easy to mock for testing.
package k8sclient

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

// Consolidated interface for k8sclient
// Add all methods from ServiceAccountClient and Client here
// If ServiceAccountClient has more methods, add them below

type Cli interface {
	UpdateServiceAccount(ctx context.Context, sa *corev1.ServiceAccount) error
	GetServiceAccount(ctx context.Context, name, namespace string) (*corev1.ServiceAccount, error)
}
