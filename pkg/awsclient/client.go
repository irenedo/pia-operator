package awsclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/go-logr/logr"
	k8sclient "github.com/irenedo/pia-operator/pkg/k8sclient"
	corev1 "k8s.io/api/core/v1"
)

// Client implements the AWSClient interface using AWS SDK
type Client struct {
	eksClient   *eks.Client
	clusterName string
	region      string
	log         logr.Logger
	KubeClient  k8sclient.DefaultServiceAccountClient
}

// NewClient creates a new AWS Pod Identity client
func NewClient(ctx context.Context, clusterName, region string, log logr.Logger) (AWSClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	eksClient := eks.NewFromConfig(cfg)

	// kubeClient must be injected after construction
	return &Client{
		eksClient:   eksClient,
		clusterName: clusterName,
		region:      region,
		log:         log,
	}, nil
}

// CreatePodIdentityAssociation creates a new AWS EKS Pod Identity Association that allows
// a Kubernetes ServiceAccount to assume an IAM role without long-lived credentials.
// Returns the association ID which is stored in the ServiceAccount's annotations.
func (c *Client) CreatePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount, roleArn, assumeRoleArn string) (string, error) {
	log := c.log.WithValues("serviceaccount", sa.Name, "namespace", sa.Namespace, "operation", "create")

	input := &eks.CreatePodIdentityAssociationInput{
		ClusterName:        aws.String(c.clusterName),
		Namespace:          aws.String(sa.Namespace),
		ServiceAccount:     aws.String(sa.Name),
		RoleArn:            aws.String(roleArn), // Base role always goes to RoleArn
		Tags: map[string]string{
			"managed-by":     "pia-operator",
			"serviceaccount": sa.Name,
			"namespace":      sa.Namespace,
			"base-role":      roleArn,
		},
	}

	// Set target role if assume role is provided
	if assumeRoleArn != "" {
		input.TargetRoleArn = aws.String(assumeRoleArn)
		input.Tags["assume-role"] = assumeRoleArn
	}

	log.Info("Creating Pod Identity Association", "roleArn", roleArn, "targetRoleArn", assumeRoleArn, "clusterName", c.clusterName)

	result, err := c.eksClient.CreatePodIdentityAssociation(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create Pod Identity Association: %w", err)
	}

	associationID := aws.ToString(result.Association.AssociationId)
	if sa.Annotations == nil {
		sa.Annotations = make(map[string]string)
	}
	sa.Annotations["pia-operator.eks.aws.com/association-id"] = associationID

	log.Info("Successfully created Pod Identity Association",
		"associationID", associationID)

	return associationID, nil
}

// UpdatePodIdentityAssociation updates an existing AWS EKS Pod Identity Association with new role ARNs.
// It finds the association by ID from ServiceAccount annotations or by searching all associations.
// Returns the association ID after successful update.
func (c *Client) UpdatePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount, roleArn, assumeRoleArn string) (string, error) {
	log := c.log.WithValues("serviceaccount", sa.Name, "namespace", sa.Namespace, "operation", "update")

	// Get the existing association ID
	associationID := sa.Annotations["pia-operator.eks.aws.com/association-id"]
	if associationID == "" {
		// Try to find the association by service account details
		association, err := c.findAssociationByServiceAccount(ctx, sa)
		if err != nil {
			return "", fmt.Errorf("failed to find existing association: %w", err)
		}
		associationID = association.ID
	}

	input := &eks.UpdatePodIdentityAssociationInput{
		ClusterName:        aws.String(c.clusterName),
		AssociationId:      aws.String(associationID),
		RoleArn:            aws.String(roleArn), // Base role always goes to RoleArn
	}

	// Set target role if assume role is provided
	if assumeRoleArn != "" {
		input.TargetRoleArn = aws.String(assumeRoleArn)
	}

	log.Info("Updating Pod Identity Association",
		"associationID", associationID,
		"roleArn", roleArn,
		"targetRoleArn", assumeRoleArn,
		"clusterName", c.clusterName)

	_, err := c.eksClient.UpdatePodIdentityAssociation(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to update Pod Identity Association: %w", err)
	}

	c.log.Info("Successfully updated Pod Identity Association",
		"associationID", associationID)

	return associationID, nil
}

// DeletePodIdentityAssociation deletes an AWS EKS Pod Identity Association for the given ServiceAccount.
// It finds the association by ID from ServiceAccount annotations or by searching all associations.
// Returns nil if the association doesn't exist (idempotent operation).
func (c *Client) DeletePodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount) error {
	// Get the existing association ID
	associationID := sa.Annotations["pia-operator.eks.aws.com/association-id"]
	if associationID == "" {
		// Try to find the association by service account details
		association, err := c.findAssociationByServiceAccount(ctx, sa)
		if err != nil {
			// If association doesn't exist, consider it already deleted
			if c.isNotFoundError(err) {
				c.log.Info("Pod Identity Association not found, considering it already deleted")
				return nil
			}
			return fmt.Errorf("failed to find existing association: %w", err)
		}
		associationID = association.ID
	}

	input := &eks.DeletePodIdentityAssociationInput{
		ClusterName:   aws.String(c.clusterName),
		AssociationId: aws.String(associationID),
	}

	c.log.Info("Deleting Pod Identity Association",
		"associationID", associationID,
		"clusterName", c.clusterName)

	_, err := c.eksClient.DeletePodIdentityAssociation(ctx, input)
	if err != nil {
		if c.isNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to delete Pod Identity Association: %w", err)
	}

	return nil
}

// GetPodIdentityAssociation retrieves an AWS EKS Pod Identity Association for the given ServiceAccount.
// It first tries using the association ID from ServiceAccount annotations, then searches all associations.
// Returns the association details or an error if not found.
func (c *Client) GetPodIdentityAssociation(ctx context.Context, sa *corev1.ServiceAccount) (*PodIdentityAssociation, error) {
	// Try to get association ID from annotations first
	associationID := sa.Annotations["pia-operator.eks.aws.com/association-id"]
	if associationID != "" {
		input := &eks.DescribePodIdentityAssociationInput{
			ClusterName:   aws.String(c.clusterName),
			AssociationId: aws.String(associationID),
		}

		result, err := c.eksClient.DescribePodIdentityAssociation(ctx, input)
		if err != nil {
			if c.isNotFoundError(err) {
				return nil, errors.New("Pod Identity Association not found")
			}
			return nil, fmt.Errorf("failed to describe Pod Identity Association: %w", err)
		}

		return c.convertToAssociation(result.Association), nil
	}

	// If no association ID, try to find by service account details
	return c.findAssociationByServiceAccount(ctx, sa)
}

// AssociationExists checks if a Pod Identity Association exists for the given ServiceAccount
func (c *Client) AssociationExists(ctx context.Context, sa *corev1.ServiceAccount) (bool, error) {
	_, err := c.GetPodIdentityAssociation(ctx, sa)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListPodIdentityAssociations lists all Pod Identity Associations for the cluster
func (c *Client) ListPodIdentityAssociations(ctx context.Context) ([]*PodIdentityAssociation, error) {
	input := &eks.ListPodIdentityAssociationsInput{
		ClusterName: aws.String(c.clusterName),
	}

	var associations []*PodIdentityAssociation
	paginator := eks.NewListPodIdentityAssociationsPaginator(c.eksClient, input)

	for paginator.HasMorePages() {
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list Pod Identity Associations: %w", err)
		}

		for _, assoc := range result.Associations {
			associations = append(associations, c.convertToAssociationSummary(&assoc))
		}
	}

	c.log.Info("Listed Pod Identity Associations", "count", len(associations))
	return associations, nil
}

// findAssociationByServiceAccount finds an association by service account details
func (c *Client) findAssociationByServiceAccount(ctx context.Context, sa *corev1.ServiceAccount) (*PodIdentityAssociation, error) {
	associations, err := c.ListPodIdentityAssociations(ctx)
	if err != nil {
		return nil, err
	}

	for _, assoc := range associations {
		if assoc.Namespace == sa.Namespace && assoc.ServiceAccountName == sa.Name {
			return assoc, nil
		}
	}

	return nil, fmt.Errorf("Pod Identity Association not found for ServiceAccount %s/%s", sa.Namespace, sa.Name)
}

// isNotFoundError checks if the error is a not found error
func (c *Client) isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var resourceNotFound *types.ResourceNotFoundException
	if errors.As(err, &resourceNotFound) {
		return true
	}

	return strings.Contains(err.Error(), "ResourceNotFoundException") ||
		strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "NotFound") ||
		strings.Contains(strings.ToLower(err.Error()), "notfound")
}

// convertToAssociation converts AWS PodIdentityAssociation to our struct
func (c *Client) convertToAssociation(assoc *types.PodIdentityAssociation) *PodIdentityAssociation {
	return &PodIdentityAssociation{
		ID:                 aws.ToString(assoc.AssociationId),
		ClusterName:        aws.ToString(assoc.ClusterName),
		Namespace:          aws.ToString(assoc.Namespace),
		ServiceAccountName: aws.ToString(assoc.ServiceAccount),
		RoleArn:            aws.ToString(assoc.RoleArn),
		Tags:               assoc.Tags,
		Status:             "ACTIVE", // Default status since field doesn't exist in current SDK
		CreatedAt:          convertTimeToString(assoc.CreatedAt),
		ModifiedAt:         convertTimeToString(assoc.ModifiedAt),
	}
}

// convertToAssociationSummary converts AWS PodIdentityAssociationSummary to our struct
func (c *Client) convertToAssociationSummary(assoc *types.PodIdentityAssociationSummary) *PodIdentityAssociation {
	return &PodIdentityAssociation{
		ID:                 aws.ToString(assoc.AssociationId),
		ClusterName:        aws.ToString(assoc.ClusterName),
		Namespace:          aws.ToString(assoc.Namespace),
		ServiceAccountName: aws.ToString(assoc.ServiceAccount),
		Tags:               make(map[string]string), // Tags not available in summary
	}
}

// convertTimeToString converts *time.Time to *string
func convertTimeToString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	timeStr := t.String()
	return &timeStr
}
