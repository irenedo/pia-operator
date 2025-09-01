package controller

import (
	"context"

	"github.com/go-logr/logr"
	awsclient "github.com/irenedo/pia-operator/internal/awsclient"
	k8sclient "github.com/irenedo/pia-operator/internal/k8sclient"
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
	K8sClient     k8sclient.ServiceAccountClient
	errorHandler  *ErrorHandler
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
		r.errorHandler = NewErrorHandler(r.Client, r.Log)
	}

	// Fetch the ServiceAccount instance
	var serviceAccount corev1.ServiceAccount
	if err := r.Get(ctx, req.NamespacedName, &serviceAccount); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			log.Info("ServiceAccount resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get ServiceAccount")
		return ctrl.Result{}, err
	}

	// AWSClient is always initialized in main.go and injected into the reconciler

	// Check if ServiceAccount is being deleted
	if serviceAccount.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &serviceAccount)
	}

	// Check if relevant annotations exist
	roleArn, hasRoleArn := serviceAccount.Annotations[PodIdentityAssociationRoleAnnotation]
	assumeRoleArn, _ := serviceAccount.Annotations[PodIdentityAssociationAssumeRoleAnnotation]

	if !hasRoleArn {
		// No relevant annotations, ensure any existing association is cleaned up
		return r.cleanupPodIdentityAssociation(ctx, &serviceAccount)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&serviceAccount, PodIdentityAssociationFinalizer) {
		controllerutil.AddFinalizer(&serviceAccount, PodIdentityAssociationFinalizer)
		if err := r.Update(ctx, &serviceAccount); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Create or update Pod Identity Association
	return r.reconcilePodIdentityAssociation(ctx, &serviceAccount, roleArn, assumeRoleArn)
}

// handleDeletion handles ServiceAccount deletion
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

// reconcilePodIdentityAssociation creates or updates a Pod Identity Association
func (r *ServiceAccountReconciler) reconcilePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount, roleArn, assumeRoleArn string) (ctrl.Result, error) {
	log := r.Log.WithValues("serviceaccount", sa.Name, "namespace", sa.Namespace)

	// Check if association already exists
	exists, err := r.AWSClient.AssociationExists(ctx, sa)
	if err != nil {
		return r.errorHandler.HandleError(ctx, sa, err, "check existing Pod Identity Association")
	}

	var associationID string
	if exists {
		// Update existing association
		associationID, err = r.AWSClient.UpdatePodIdentityAssociation(ctx, sa, roleArn, assumeRoleArn)
		if err != nil {
			return r.errorHandler.HandleError(ctx, sa, err, "update Pod Identity Association")
		}
		if assumeRoleArn != "" {
			log.Info("Successfully updated Pod Identity Association", "roleArn", roleArn, "targetRoleArn", assumeRoleArn, "associationID", associationID)
		} else {
			log.Info("Successfully updated Pod Identity Association", "roleArn", roleArn, "associationID", associationID)
		}
	} else {
		// Create new association
		associationID, err = r.AWSClient.CreatePodIdentityAssociation(ctx, sa, roleArn, assumeRoleArn)
		if err != nil {
			return r.errorHandler.HandleError(ctx, sa, err, "create Pod Identity Association")
		}
		if assumeRoleArn != "" {
			log.Info("Successfully created Pod Identity Association", "roleArn", roleArn, "targetRoleArn", assumeRoleArn, "associationID", associationID)
		} else {
			log.Info("Successfully created Pod Identity Association", "roleArn", roleArn, "associationID", associationID)
		}
	}

	// Next step: update ServiceAccount manifest with association-id annotation using associationID
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

	// Mark success and reset retry count
	r.errorHandler.MarkSuccess(ctx, sa, "Pod Identity Association ready")
	return ctrl.Result{}, nil
}

// deletePodIdentityAssociation deletes a Pod Identity Association
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

// cleanupPodIdentityAssociation removes association when annotations are removed
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
	if err := r.Update(ctx, sa); err != nil {
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
				oldVal := e.ObjectOld.GetAnnotations()[PodIdentityAssociationRoleAnnotation]
				newVal := e.ObjectNew.GetAnnotations()[PodIdentityAssociationRoleAnnotation]
				return oldVal != newVal
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
