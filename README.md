# PIA Operator


A Kubernetes operator that manages [AWS Pod Identity](https://docs.aws.amazon.com/eks/latest/userguide/pod-identity.html) Associations based on service account annotations.

## Overview

The PIA (Pod Identity Association) Operator automatically creates, updates, and deletes AWS Pod Identity Associations when ServiceAccount resources are annotated with specific AWS IAM role information.

## Features

- **Automatic Management**: Watches ServiceAccount resources for annotation changes
- **AWS Integration**: Creates/updates/deletes EKS Pod Identity Associations via AWS API
- **Role Assumption**: Supports role assumption through `pia-operator.eks.aws.com/assume-role` annotation
- **Cleanup**: Automatically removes associations when annotations are deleted
- **Error Handling**: Validates required annotations and provides clear error messages

## Annotations

The operator responds to the following annotations on ServiceAccount resources:

### Required Annotations

- `pia-operator.eks.aws.com/role`: The ARN of the AWS IAM role to associate with the pod

### Optional Annotations

- `pia-operator.eks.aws.com/assume-role`: The ARN of an AWS IAM role to assume. When set, this role will be used instead of the base role.

## Behavior

### Creating Pod Identity Associations

When a ServiceAccount is annotated with `pia-operator.eks.aws.com/role`:

1. The operator validates that the required annotation is present
2. Creates an AWS Pod Identity Association linking the ServiceAccount to the specified IAM role
3. If `pia-operator.eks.aws.com/assume-role` is also present, uses that role for the association
4. Adds a finalizer to ensure proper cleanup

### Updating Pod Identity Associations

When annotations are modified:

1. The operator detects the change
2. Updates the existing Pod Identity Association with the new role information

### Deleting Pod Identity Associations

When annotations are removed or the ServiceAccount is deleted:

1. The operator removes the Pod Identity Association from AWS
2. Removes the finalizer from the ServiceAccount

### Error Conditions

The operator will log errors in the following cases:

- `pia-operator.eks.aws.com/assume-role` is set but `pia-operator.eks.aws.com/role` is missing
- AWS API calls fail (insufficient permissions, invalid role ARNs, etc.)

## Prerequisites

1. **EKS Cluster**: The operator must run on an EKS cluster with Pod Identity enabled
2. **AWS Permissions**: The operator's service account must have permissions to manage Pod Identity Associations
3. **IAM Roles**: The roles specified in annotations must exist and have appropriate trust policies

## Installation

### Option 1: Using kubectl

```bash
# Apply the operator manifests
kubectl apply -f config/

# Verify the operator is running
kubectl get pods -n pia-operator-system
```

### Option 2: Using Kustomize

```bash
# Deploy using kustomize
kubectl apply -k config/default/
```

## Configuration

### Environment Variables

The operator requires the following environment variables:

- `AWS_REGION`: The AWS region where your EKS cluster is running
- `CLUSTER_NAME`: The name of your EKS cluster

These can be set in the deployment manifest or through command-line flags:

```bash
./manager --aws-region=us-west-2 --cluster-name=my-cluster
```

### AWS Permissions

The operator's service account needs the following AWS IAM permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "eks:CreatePodIdentityAssociation",
                "eks:UpdatePodIdentityAssociation",
                "eks:DeletePodIdentityAssociation",
                "eks:DescribePodIdentityAssociation",
                "eks:ListPodIdentityAssociations"
            ],
            "Resource": "*"
        }
    ]
}
```

## Usage Examples

### Basic Example

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-service-account
  namespace: default
  annotations:
    pia-operator.eks.aws.com/role: "arn:aws:iam::123456789012:role/MyPodRole"
```

### With Role Assumption

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-service-account
  namespace: default
  annotations:
    pia-operator.eks.aws.com/role: "arn:aws:iam::123456789012:role/MyPodRole"
    pia-operator.eks.aws.com/assume-role: "arn:aws:iam::123456789012:role/AssumedRole"
```

### Using in Pod Specifications

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
  namespace: default
spec:
  serviceAccountName: my-service-account
  containers:
  - name: my-container
    image: my-image:latest
    # This pod will automatically have access to the AWS role
    # specified in the ServiceAccount annotations
```

## Development


### Prerequisites

- Go 1.21+
- Docker
- kubectl
- Access to an EKS cluster
- [Taskfile](https://taskfile.dev) (Task runner)

### Installing Taskfile

Taskfile is a modern task runner for Go projects. To install Taskfile:

**macOS (Homebrew):**
```bash
brew install go-task/tap/go-task
```

**Other platforms:**
See [Taskfile installation docs](https://taskfile.dev/#/installation) for instructions.

### Building and Running with Taskfile

```bash
# Build the binary
task build

# Build the Docker image
task docker-build

# Run tests
task test

# Install dependencies
task deps

# Run the operator locally
task run AWS_REGION=us-west-2 CLUSTER_NAME=my-cluster
```

# Deploy to Kubernetes
task deploy IMG=pia-operator:latest

# Uninstall
task undeploy

## Troubleshooting

### Common Issues

1. **"Missing required annotation" errors**: Ensure `pia-operator.eks.aws.com/role` is set when using `pia-operator.eks.aws.com/assume-role`

2. **AWS permission errors**: Verify the operator's service account has the required AWS IAM permissions

3. **Role not found errors**: Ensure the IAM roles specified in annotations exist and have proper trust policies

### Logs

Check the operator logs for detailed error information:

```bash
kubectl logs -n pia-operator-system deployment/pia-operator-controller-manager
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request
