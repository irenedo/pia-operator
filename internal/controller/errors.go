package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Retry and backoff configuration
	maxRetryAttempts = 5
	baseRetryDelay   = 30 * time.Second
	maxRetryDelay    = 5 * time.Minute

	// Status condition types
	conditionTypePodIdentityReady = "PodIdentityReady"
	conditionReasonReconciling    = "Reconciling"
	conditionReasonReady          = "Ready"
	conditionReasonError          = "Error"
	conditionReasonRetrying       = "Retrying"
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

// ClassifyError determines how to handle different error types
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

// CalculateBackoff calculates exponential backoff delay
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

// GetRetryCount gets the current retry count from annotations
func (eh *ErrorHandler) GetRetryCount(sa *corev1.ServiceAccount) int {
	key := sa.Namespace + "/" + sa.Name
	return eh.retryCounts[key]
}

// SetRetryCount sets the retry count in annotations
func (eh *ErrorHandler) SetRetryCount(ctx context.Context, sa *corev1.ServiceAccount, count int) {
	key := sa.Namespace + "/" + sa.Name
	eh.retryCounts[key] = count
}

// UpdateServiceAccountStatus updates the ServiceAccount status with current condition
func (eh *ErrorHandler) UpdateServiceAccountStatus(sa *corev1.ServiceAccount, conditionType, reason, message string, status metav1.ConditionStatus) {
	log := eh.log.WithValues("serviceaccount", sa.Name, "namespace", sa.Namespace)

	// Note: ServiceAccount doesn't have a status field by default
	// This is for logging purposes. In a real implementation, you might
	// want to use events or a custom status field via CRD
	log.Info("Status update", "condition", conditionType, "reason", reason, "message", message, "status", status)
}

// HandleError provides a unified way to handle errors with appropriate retry logic
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
		eh.UpdateServiceAccountStatus(sa, conditionTypePodIdentityReady, conditionReasonError,
			fmt.Sprintf("Permanent error in %s", operation), metav1.ConditionFalse)
		return ctrl.Result{}, nil

	case ErrorTransient:
		retryCount := eh.GetRetryCount(sa)
		if retryCount >= maxRetryAttempts {
			log.Error(err, "Max retry attempts reached", "retryCount", retryCount, "operation", operation)
			eh.UpdateServiceAccountStatus(sa, conditionTypePodIdentityReady, conditionReasonError,
				fmt.Sprintf("Max retries reached for %s", operation), metav1.ConditionFalse)
			return ctrl.Result{RequeueAfter: maxRetryDelay}, nil
		}

		eh.SetRetryCount(ctx, sa, retryCount+1)
		delay := eh.CalculateBackoff(retryCount)
		log.Info("Transient error, retrying with backoff", "delay", delay, "retryCount", retryCount, "error", err)
		eh.UpdateServiceAccountStatus(sa, conditionTypePodIdentityReady, conditionReasonRetrying,
			fmt.Sprintf("Retrying %s (attempt %d/%d)", operation, retryCount+1, maxRetryAttempts), metav1.ConditionFalse)
		return ctrl.Result{RequeueAfter: delay}, nil

	default:
		log.Info("Retryable error, retrying immediately", "error", err)
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

	default:
		log.Info("Retryable error during deletion, retrying immediately", "error", err)
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
	eh.UpdateServiceAccountStatus(sa, conditionTypePodIdentityReady, conditionReasonReady, message, metav1.ConditionTrue)
}
