package k8sclient

import (
	context "context"
	corev1 "k8s.io/api/core/v1"
)

type Client interface {
	UpdateServiceAccount(ctx context.Context, sa *corev1.ServiceAccount) error
}
