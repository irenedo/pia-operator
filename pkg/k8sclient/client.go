package k8sclient

import (
	context "context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DefaultServiceAccountClient struct {
	Client client.Client
}

func (c *DefaultServiceAccountClient) UpdateServiceAccount(ctx context.Context, sa *corev1.ServiceAccount) error {
	return c.Client.Update(ctx, sa)
}

func (c *DefaultServiceAccountClient) GetServiceAccount(ctx context.Context, namespace, name string) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{}
	err := c.Client.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, sa)
	return sa, err
}

// NewClient returns a Cli implementation
func NewClient(c client.Client) Cli {
	return &DefaultServiceAccountClient{Client: c}
}
