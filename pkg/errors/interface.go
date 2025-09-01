// Package errors provides intelligent error handling and retry logic for Kubernetes controllers.
//
// This package implements a comprehensive error handling system that classifies errors into
// different categories (permanent, transient, retryable) and applies appropriate retry
// strategies with exponential backoff. It's designed specifically for controller operations
// that interact with external services like AWS APIs.
//
// The error handler tracks retry counts per resource and provides specialized handling
// for deletion operations to prevent blocking resource cleanup.
package errors

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ErrorHandlerInterface defines the contract for error handling in controllers
type ErrorHandlerInterface interface {
	HandleError(ctx context.Context, sa *corev1.ServiceAccount, err error, operation string) (ctrl.Result, error)
	HandleDeletionError(ctx context.Context, sa *corev1.ServiceAccount, err error, operation string) (ctrl.Result, error)
	MarkSuccess(ctx context.Context, sa *corev1.ServiceAccount, message string)
	ResetRetryCount(ctx context.Context, sa *corev1.ServiceAccount)
}
