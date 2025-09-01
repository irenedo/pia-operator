package errors

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Retry and backoff configuration
	maxRetryAttempts = 5
	baseRetryDelay   = 30 * time.Second
	maxRetryDelay    = 5 * time.Minute
)

// ErrorClassification represents different types of errors
type ErrorClassification int

const (
	ErrorPermanent ErrorClassification = iota // Don't retry
	ErrorTransient                            // Retry with backoff
	ErrorRetryable                            // Retry immediately
)

// ErrorHandler provides error handling utilities for the controller
type ErrorHandler struct {
	client      client.Client
	log         logr.Logger
	retryCounts map[string]int // key: namespace/name
}

// NewErrorHandler creates a new ErrorHandler instance
func NewErrorHandler(client client.Client, log logr.Logger) *ErrorHandler {
	return &ErrorHandler{
		client:      client,
		log:         log,
		retryCounts: make(map[string]int),
	}
}

// ClassifyError analyzes an error and returns its classification (permanent, transient, or retryable).
// This determines the retry strategy: permanent errors are not retried, transient use backoff, retryable retry immediately.
func (eh *ErrorHandler) ClassifyError(err error) ErrorClassification {
	if err == nil {
		return ErrorRetryable
	}

	// Kubernetes API errors
	if errors.IsNotFound(err) || errors.IsInvalid(err) || errors.IsBadRequest(err) {
		return ErrorPermanent
	}

	// Rate limiting and service unavailable
	if errors.IsTooManyRequests(err) || errors.IsServiceUnavailable(err) || errors.IsTimeout(err) {
		return ErrorTransient
	}

	// Conflict errors (optimistic locking) - retry immediately
	if errors.IsConflict(err) {
		return ErrorRetryable
	}

	// Default to transient for unknown errors
	return ErrorTransient
}

// CalculateBackoff calculates exponential backoff delay based on retry count, starting from baseRetryDelay.
// The delay doubles with each retry up to maxRetryDelay to avoid overwhelming external services.
func (eh *ErrorHandler) CalculateBackoff(retryCount int) time.Duration {
	delay := baseRetryDelay
	for i := 0; i < retryCount && delay < maxRetryDelay; i++ {
		delay *= 2
	}
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	return delay
}

// GetRetryCount gets the current retry count
func (eh *ErrorHandler) GetRetryCount(sa *corev1.ServiceAccount) int {
	key := sa.Namespace + "/" + sa.Name
	return eh.retryCounts[key]
}

// SetRetryCount sets the retry count
func (eh *ErrorHandler) SetRetryCount(ctx context.Context, sa *corev1.ServiceAccount, count int) {
	key := sa.Namespace + "/" + sa.Name
	eh.retryCounts[key] = count
}

// HandleError provides unified error handling with intelligent retry logic based on error classification.
// It tracks retry counts per resource and applies exponential backoff for transient errors.
// Returns appropriate controller.Result for requeuing with immediate retry, backoff delay, or no retry.
func (eh *ErrorHandler) HandleError(ctx context.Context, sa *corev1.ServiceAccount, err error, operation string) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}

	log := eh.log.WithValues("serviceaccount", sa.Name, "namespace", sa.Namespace, "operation", operation)
	log.Error(err, "Operation failed")

	errorClass := eh.ClassifyError(err)
	switch errorClass {
	case ErrorPermanent:
		log.Info("Permanent error, not retrying", "error", err)
		return ctrl.Result{}, nil

	case ErrorTransient:
		retryCount := eh.GetRetryCount(sa)
		if retryCount >= maxRetryAttempts {
			log.Error(err, "Max retry attempts reached", "retryCount", retryCount, "operation", operation)
			return ctrl.Result{RequeueAfter: maxRetryDelay}, nil
		}

		eh.SetRetryCount(ctx, sa, retryCount+1)
		delay := eh.CalculateBackoff(retryCount)
		log.Info("Transient error, retrying with backoff", "delay", delay, "retryCount", retryCount, "error", err)
		return ctrl.Result{RequeueAfter: delay}, nil

	case ErrorRetryable:
		log.Info("Retryable error, retrying immediately", "error", err)
		return ctrl.Result{Requeue: true}, nil

	default:
		// This should never happen, but handle gracefully
		log.Info("Unknown error classification, retrying immediately", "error", err)
		return ctrl.Result{Requeue: true}, nil
	}
}

// HandleDeletionError provides specialized error handling for deletion operations
// It's more permissive since we don't want to block resource deletion
func (eh *ErrorHandler) HandleDeletionError(ctx context.Context, sa *corev1.ServiceAccount, err error, operation string) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}

	log := eh.log.WithValues("serviceaccount", sa.Name, "namespace", sa.Namespace, "operation", operation)
	log.Error(err, "Deletion operation failed")

	errorClass := eh.ClassifyError(err)
	switch errorClass {
	case ErrorPermanent:
		// For permanent errors during deletion, log the error but continue
		// to avoid blocking ServiceAccount deletion
		log.Info("Permanent error during deletion, continuing anyway", "error", err)
		return ctrl.Result{}, nil

	case ErrorTransient:
		retryCount := eh.GetRetryCount(sa)
		if retryCount >= maxRetryAttempts {
			log.Error(err, "Max retry attempts reached for deletion, continuing anyway", "retryCount", retryCount)
			return ctrl.Result{}, nil
		}

		eh.SetRetryCount(ctx, sa, retryCount+1)
		delay := eh.CalculateBackoff(retryCount)
		log.Info("Transient error during deletion, retrying with backoff", "delay", delay, "retryCount", retryCount, "error", err)
		return ctrl.Result{RequeueAfter: delay}, nil

	case ErrorRetryable:
		log.Info("Retryable error during deletion, retrying immediately", "error", err)
		return ctrl.Result{Requeue: true}, nil

	default:
		// This should never happen, but handle gracefully
		log.Info("Unknown error classification during deletion, retrying immediately", "error", err)
		return ctrl.Result{Requeue: true}, nil
	}
}

// ResetRetryCount resets the retry count on successful operations
func (eh *ErrorHandler) ResetRetryCount(ctx context.Context, sa *corev1.ServiceAccount) {
	key := sa.Namespace + "/" + sa.Name
	eh.retryCounts[key] = 0
}

// MarkSuccess marks an operation as successful and resets retry count
func (eh *ErrorHandler) MarkSuccess(ctx context.Context, sa *corev1.ServiceAccount, message string) {
	eh.ResetRetryCount(ctx, sa)
}
