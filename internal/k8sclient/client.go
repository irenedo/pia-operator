package k8sclient

import (
	context "context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceAccountClient abstracts ServiceAccount operations for testability
type ServiceAccountClient interface {
	UpdateServiceAccount(ctx context.Context, sa *corev1.ServiceAccount) error
}

type DefaultServiceAccountClient struct {
	Client client.Client
}

func (c *DefaultServiceAccountClient) UpdateServiceAccount(ctx context.Context, sa *corev1.ServiceAccount) error {
	return c.Client.Update(ctx, sa)
}

// NewClient returns a ServiceAccountClient implementation
func NewClient(c client.Client) ServiceAccountClient {
	return &DefaultServiceAccountClient{Client: c}
}
