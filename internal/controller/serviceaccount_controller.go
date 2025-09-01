// reconcilePodIdentityAssociation ensures that the ServiceAccount has the correct Pod Identity Association in AWS.
// It checks if an association exists and creates or updates it as needed, updating Kubernetes annotations with the association ID.
// This method guarantees that the ServiceAccount's AWS role mapping is always consistent with its annotations.
// ServiceAccountReconciler is a Kubernetes controller that manages Pod Identity Associations for ServiceAccount resources.
// It ensures that ServiceAccounts annotated with specific AWS IAM role ARNs are correctly associated with AWS Pod Identity,
// handling creation, update, and deletion of these associations as ServiceAccounts are created, updated, or deleted.
//
// The reconciler watches for ServiceAccounts with the following annotations:
//   - pia-operator.eks.aws.com/role: Specifies the AWS IAM role ARN to associate.
//   - pia-operator.eks.aws.com/assume-role: (Optional) Specifies a target role ARN for role assumption.
//
// When a ServiceAccount is annotated, the controller:
//   - Adds a finalizer to ensure cleanup on deletion.
//   - Creates or updates the Pod Identity Association in AWS.
//   - Stores the association ID in the ServiceAccount's annotations.
//
// On deletion or annotation removal, the controller:
//   - Deletes the Pod Identity Association in AWS.
//   - Removes related annotations and the finalizer from the ServiceAccount.
//
// The controller uses custom error handling and metrics to track reconciliation status and errors.
// It is designed to be robust against transient errors and supports retry logic via controller-runtime mechanisms.
//
// RBAC permissions are required for managing ServiceAccounts and their finalizers/status fields.
package controller

import (
	"context"

	"github.com/go-logr/logr"
	awsclient "github.com/irenedo/pia-operator/pkg/awsclient"
	errorhandling "github.com/irenedo/pia-operator/pkg/errors"
	k8sclient "github.com/irenedo/pia-operator/pkg/k8sclient"
	metric "github.com/irenedo/pia-operator/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// Annotations for Pod Identity Association
	PodIdentityAssociationRoleAnnotation       = "pia-operator.eks.aws.com/role"
	PodIdentityAssociationAssumeRoleAnnotation = "pia-operator.eks.aws.com/assume-role"

	// Finalizer for cleanup
	PodIdentityAssociationFinalizer = "pia-operator.eks.aws.com/finalizer"

	// Annotation for storing Pod Identity Association ID
	PodIdentityAssociationIDAnnotation = "pia-operator.eks.aws.com/association-id"
)

// ServiceAccountReconciler reconciles a ServiceAccount object
type ServiceAccountReconciler struct {
	client.Client // for controller-runtime
	Log           logr.Logger
	Scheme        *runtime.Scheme
	AWSRegion     string
	ClusterName   string
	AWSClient     awsclient.AWSClient
	K8sClient     k8sclient.Cli
	errorHandler  errorhandling.ErrorHandlerInterface
}

// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("serviceaccount", req.NamespacedName)

	// Initialize error handler if not already done
	if r.errorHandler == nil {
		r.errorHandler = errorhandling.NewErrorHandler(r.Client, r.Log)
	}

	// Fetch the ServiceAccount instance
	serviceAccount, err := r.K8sClient.GetServiceAccount(ctx, req.Namespace, req.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			log.Info("ServiceAccount resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ServiceAccount")
		return ctrl.Result{}, err
	}

	// Check if ServiceAccount is being deleted
	if serviceAccount.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, serviceAccount)
	}

	// Check if relevant annotations exist
	roleArn, hasRoleArn := serviceAccount.Annotations[PodIdentityAssociationRoleAnnotation]
	assumeRoleArn := serviceAccount.Annotations[PodIdentityAssociationAssumeRoleAnnotation]

	if !hasRoleArn {
		// No relevant annotations, ensure any existing association is cleaned up
		return r.cleanupPodIdentityAssociation(ctx, serviceAccount)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(serviceAccount, PodIdentityAssociationFinalizer) {
		controllerutil.AddFinalizer(serviceAccount, PodIdentityAssociationFinalizer)
		if err := r.Update(ctx, serviceAccount); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Create or update Pod Identity Association
	return r.reconcilePodIdentityAssociation(ctx, serviceAccount, roleArn, assumeRoleArn)
}

// reconcilePodIdentityAssociation creates or updates a Pod Identity Association in AWS EKS
// for the given ServiceAccount, establishing the connection between the Kubernetes ServiceAccount
// and the specified IAM role ARN to enable pod-level IAM permissions.
func (r *ServiceAccountReconciler) reconcilePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount, roleArn, assumeRoleArn string) (ctrl.Result, error) {
	log := r.Log.WithValues("serviceaccount", sa.Name, "namespace", sa.Namespace)

	exists, err := r.AWSClient.AssociationExists(ctx, sa)
	if err != nil {
		return r.errorHandler.HandleError(ctx, sa, err, "check existing Pod Identity Association")
	}

	var associationID string
	var op string
	if exists {
		associationID, err = r.updatePodIdentityAssociation(ctx, sa, roleArn, assumeRoleArn, log)
		op = "update"
	} else {
		associationID, err = r.createPodIdentityAssociation(ctx, sa, roleArn, assumeRoleArn, log)
		op = "create"
	}
	if err != nil {
		metric.IncAssociationError(op)
		return ctrl.Result{}, err
	}

	if associationID != "" {
		if sa.Annotations == nil {
			sa.Annotations = make(map[string]string)
		}
		sa.Annotations[PodIdentityAssociationIDAnnotation] = associationID
		if err := r.K8sClient.UpdateServiceAccount(ctx, sa); err != nil {
			log.Error(err, "Failed to update ServiceAccount with association ID annotation")
			return r.errorHandler.HandleError(ctx, sa, err, "update ServiceAccount annotation")
		}
	}

	// Mark success
	r.errorHandler.MarkSuccess(ctx, sa, "Pod Identity Association ready")

	metric.SetAssociationsManaged(1)
	return ctrl.Result{}, nil
}

// updatePodIdentityAssociation updates an existing Pod Identity Association in AWS EKS
// with new role ARN configuration, ensuring the ServiceAccount maintains proper
// IAM role binding while preserving the existing association ID.
func (r *ServiceAccountReconciler) updatePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount, roleArn, assumeRoleArn string, log logr.Logger) (string, error) {
	associationID, err := r.AWSClient.UpdatePodIdentityAssociation(ctx, sa, roleArn, assumeRoleArn)
	if err != nil {
		result, handleErr := r.errorHandler.HandleError(ctx, sa, err, "update Pod Identity Association")
		if handleErr != nil {
			return "", handleErr
		}
		if result.Requeue || result.RequeueAfter > 0 {
			return "", err // Return original error to trigger requeue
		}
	}
	r.logAssociationSuccess(log, "updated", roleArn, assumeRoleArn, associationID)
	return associationID, nil
}

// createPodIdentityAssociation creates a new Pod Identity Association in AWS EKS
// linking the ServiceAccount to the specified IAM role ARN.
func (r *ServiceAccountReconciler) createPodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount, roleArn, assumeRoleArn string, log logr.Logger) (string, error) {
	associationID, err := r.AWSClient.CreatePodIdentityAssociation(ctx, sa, roleArn, assumeRoleArn)
	if err != nil {
		result, handleErr := r.errorHandler.HandleError(ctx, sa, err, "create Pod Identity Association")
		if handleErr != nil {
			return "", handleErr
		}
		if result.Requeue || result.RequeueAfter > 0 {
			return "", err // Return original error to trigger requeue
		}
	}
	r.logAssociationSuccess(log, "created", roleArn, assumeRoleArn, associationID)
	return associationID, nil
}

// logAssociationSuccess logs successful Pod Identity Association operations
// with role ARN details and association ID for audit purposes.
func (r *ServiceAccountReconciler) logAssociationSuccess(log logr.Logger, operation, roleArn, assumeRoleArn, associationID string) {
	if assumeRoleArn != "" {
		log.Info("Successfully "+operation+" Pod Identity Association", "roleArn", roleArn, "targetRoleArn", assumeRoleArn, "associationID", associationID)
	} else {
		log.Info("Successfully "+operation+" Pod Identity Association", "roleArn", roleArn, "associationID", associationID)
	}
}

// handleDeletion handles the deletion process for a ServiceAccount resource.
// It checks for the presence of the PodIdentityAssociationFinalizer, deletes the associated Pod Identity Association if necessary,
// removes the finalizer, and updates the ServiceAccount. Errors encountered during these steps are handled and returned appropriately.
func (r *ServiceAccountReconciler) handleDeletion(ctx context.Context, sa *corev1.ServiceAccount) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(sa, PodIdentityAssociationFinalizer) {
		// Delete Pod Identity Association
		if err := r.deletePodIdentityAssociation(ctx, sa); err != nil {
			return r.errorHandler.HandleDeletionError(ctx, sa, err, "delete Pod Identity Association")
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(sa, PodIdentityAssociationFinalizer)
		if err := r.Update(ctx, sa); err != nil {
			return r.errorHandler.HandleDeletionError(ctx, sa, err, "remove finalizer")
		}
	}

	return ctrl.Result{}, nil
}

// deletePodIdentityAssociation removes the Pod Identity Association from AWS EKS
// for the given ServiceAccount, breaking the IAM role binding.
func (r *ServiceAccountReconciler) deletePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount) error {
	log := r.Log.WithValues("serviceaccount", sa.Name, "namespace", sa.Namespace)

	if err := r.AWSClient.DeletePodIdentityAssociation(ctx, sa); err != nil {
		return err
	}

	// Remove Pod Identity Association annotations
	if sa.Annotations != nil {
		delete(sa.Annotations, PodIdentityAssociationAssumeRoleAnnotation)
		delete(sa.Annotations, PodIdentityAssociationIDAnnotation)
		if err := r.K8sClient.UpdateServiceAccount(ctx, sa); err != nil {
			log.Error(err, "Failed to remove Pod Identity Association annotations from ServiceAccount")
			return err
		}
	}

	log.Info("Successfully deleted Pod Identity Association and removed related annotations")
	return nil
}

// cleanupPodIdentityAssociation removes the Pod Identity Association and finalizer
// when the ServiceAccount no longer has the required annotations.
func (r *ServiceAccountReconciler) cleanupPodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(sa, PodIdentityAssociationFinalizer) {
		// No finalizer means no association to clean up
		return ctrl.Result{}, nil
	}

	// Delete the association
	if err := r.deletePodIdentityAssociation(ctx, sa); err != nil {
		return r.errorHandler.HandleDeletionError(ctx, sa, err, "cleanup Pod Identity Association")
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(sa, PodIdentityAssociationFinalizer)
	if err := r.K8sClient.UpdateServiceAccount(ctx, sa); err != nil {
		return r.errorHandler.HandleDeletionError(ctx, sa, err, "remove finalizer during cleanup")
	}

	// Reset retry count on successful cleanup
	r.errorHandler.ResetRetryCount(ctx, sa)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ServiceAccount{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldRoleArn := e.ObjectOld.GetAnnotations()[PodIdentityAssociationRoleAnnotation]
				newRoleArn := e.ObjectNew.GetAnnotations()[PodIdentityAssociationRoleAnnotation]
				oldAssumeRoleArn := e.ObjectOld.GetAnnotations()[PodIdentityAssociationAssumeRoleAnnotation]
				newAssumeRoleArn := e.ObjectNew.GetAnnotations()[PodIdentityAssociationAssumeRoleAnnotation]
				return oldRoleArn != newRoleArn || oldAssumeRoleArn != newAssumeRoleArn
			},
			CreateFunc: func(e event.CreateEvent) bool {
				annotations := e.Object.GetAnnotations()
				_, hasRoleArn := annotations[PodIdentityAssociationRoleAnnotation]
				_, hasAssumeRoleArn := annotations[PodIdentityAssociationAssumeRoleAnnotation]
				hasFinalizer := controllerutil.ContainsFinalizer(e.Object, PodIdentityAssociationFinalizer)
				return hasRoleArn || hasAssumeRoleArn || hasFinalizer
			},
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		}).
		Complete(r)
}
