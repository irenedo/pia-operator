# PIA Operator

A Kubernetes operator that manages [AWS Pod Identity](https://docs.aws.amazon.com/eks/latest/userguide/pod-identity.html) Associations based on service account annotations.

## Overview

The PIA (Pod Identity Association) Operator automatically creates, updates, and deletes AWS Pod Identity Associations when ServiceAccount resources are annotated with specific AWS IAM role information. This enables seamless integration between Kubernetes workloads and AWS services through proper IAM role assignment.

## Motivation

In large-scale enterprise environments with dozens of EKS clusters distributed across separate AWS accounts, managing Pod Identity Associations becomes a significant operational challenge. Traditional approaches require platform teams to manually create and maintain Pod Identity Associations for every application across multiple clusters and accounts, creating bottlenecks and reducing development velocity.

Our infrastructure design addresses this complexity by providing each EKS cluster with a default general-purpose IAM role that has permission to assume destination roles in AWS accounts owned by various teams and business units. However, this approach still left a gap: applications could not autonomously establish the specific Pod Identity Associations required to link their ServiceAccounts to the appropriate target roles in destination accounts.

The PIA Operator solves this architectural challenge by enabling a self-service model where:

- **Application teams** define their AWS identity requirements directly in their Kubernetes manifests through ServiceAccount annotations
- **Destination account managers** (the teams owning the target AWS accounts) retain full control over their IAM roles and policies
- **Platform teams** are relieved from the operational burden of managing thousands of Pod Identity Associations across multiple clusters and accounts

This approach transforms Pod Identity Association management from a centralized, manually-intensive process into a distributed, declarative system. Application teams can specify their required roles in their GitOps repositories, while the operator automatically creates the necessary associations. Destination account managers maintain sovereignty over their AWS resources by controlling role trust policies and permissions, ensuring security boundaries remain intact.

The result is a scalable architecture that eliminates coordination overhead between platform teams, application teams, and account owners, while maintaining security best practices and enabling autonomous deployment workflows.

## Features

- **Automatic Management**: Watches ServiceAccount resources for annotation changes
- **AWS Integration**: Creates/updates/deletes EKS Pod Identity Associations via AWS API
- **Role Assumption**: Supports role assumption through `pia-operator.eks.aws.com/assume-role` annotation
- **Cleanup**: Automatically removes associations when annotations are deleted
- **Metrics**: Exposes Prometheus metrics for monitoring association management
- **Security**: Runs with minimal privileges and security best practices

## Prerequisites

1. **EKS Cluster**: The operator must run on an EKS cluster with Pod Identity enabled
2. **AWS Permissions**: The operator's service account must have permissions to manage Pod Identity Associations
3. **IAM Roles**: The roles specified in annotations must exist and have appropriate trust policies
4. **Kubernetes Version**: Kubernetes 1.19+ (recommended 1.21+)

## Installation

### Option 1: Using Helm (Recommended)

Add the chart repository and install:

```bash
# Install with required cluster name
helm install pia-operator ./charts/pia-operator \
  --set operator.clusterName=my-eks-cluster \
  --create-namespace \
  --namespace pia-operator-system

# Install with custom configuration
helm install pia-operator ./charts/pia-operator \
  --set operator.clusterName=my-eks-cluster \
  --set operator.aws.region=us-west-2 \
  --set operator.devMode=true \
  --set deployment.replicas=2 \
  --create-namespace \
  --namespace pia-operator-system
```

#### Helm Configuration Values

The following table lists the configurable parameters of the PIA Operator chart:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.aws.region` | AWS region where the EKS cluster is running | `eu-west-1` |
| `operator.clusterName` | EKS cluster name (required) | `""` |
| `operator.metricsBindAddress` | Address for metrics endpoint | `:8080` |
| `operator.healthProbeBindAddress` | Address for health probe endpoint | `:8081` |
| `operator.devMode` | Enable development logging mode | `false` |
| `operator.leaderElection` | Enable leader election for HA | `false` |
| `image.repository` | Container image repository | `renedo/pia-operator` |
| `image.tag` | Container image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `deployment.replicas` | Number of operator replicas | `1` |
| `deployment.resources.limits.cpu` | CPU limit | `500m` |
| `deployment.resources.limits.memory` | Memory limit | `128Mi` |
| `deployment.resources.requests.cpu` | CPU request | `10m` |
| `deployment.resources.requests.memory` | Memory request | `64Mi` |
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.annotations` | Service account annotations | `{}` |
| `rbac.create` | Create RBAC resources | `true` |
| `namespace.create` | Create namespace | `false` |

### Option 2: Using Kustomize

```bash
# Deploy using kustomize
kubectl apply -k config/default/

# Verify the operator is running
kubectl get pods -n pia-operator-system
```

### Option 3: Using kubectl

```bash
# Apply the operator manifests directly
kubectl apply -f config/

# Verify the operator is running
kubectl get pods -n pia-operator-system
```

## Configuration

### Required Parameters

The operator requires the following parameters to function:

1. **Cluster Name**: Must be provided via `--cluster-name` flag or Helm value
2. **AWS Region**: Can be provided via `--aws-region` flag (defaults to `eu-west-1`)

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

### Setting up AWS Permissions

1. **Create IAM Role**: Create an IAM role with the above permissions

2. **Configure Pod Identity Trust Policy**: The IAM role needs a trust policy that allows EKS Pod Identity to assume it:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": "pods.eks.amazonaws.com"
            },
            "Action": [
                "sts:AssumeRole",
                "sts:TagSession"
            ]
        }
    ]
}
```

3. **Create Pod Identity Association**: You must create a Pod Identity Association in your EKS cluster to link the operator's service account with the IAM role. This **cannot** be done by the operator itself since it needs this permission to function.

```bash
# Using AWS CLI
aws eks create-pod-identity-association \
    --cluster-name YOUR_CLUSTER_NAME \
    --namespace pia-operator-system \
    --service-account YOUR_SERVICE_ACCOUNT_NAME \
    --role-arn arn:aws:iam::ACCOUNT_ID:role/pia-operator-role

# Or using eksctl
eksctl create podidentityassociation \
    --cluster YOUR_CLUSTER_NAME \
    --namespace pia-operator-system \
    --service-account-name YOUR_SERVICE_ACCOUNT_NAME \
    --role-arn arn:aws:iam::ACCOUNT_ID:role/pia-operator-role

# Or through AWS Console:
# EKS Console → Clusters → Your Cluster → Access → Pod Identity associations → Create association
```

**Important**: The service account name will be generated by Helm (default: `pia-operator-controller-manager`) or you can specify it with `--set serviceAccount.name=your-custom-name`.

## Annotations

The operator responds to the following annotations on ServiceAccount resources:

### Required Annotations

- `pia-operator.eks.aws.com/role`: The ARN of the AWS IAM role to associate with the pod

### Optional Annotations

- `pia-operator.eks.aws.com/assume-role`: The ARN of an AWS IAM role to assume. When set, this role will be used instead of the base role.
- `pia-operator.eks.aws.com/tagging`: Boolean value to control session tags (default: `true`). Set to `false` to disable session tags in the Pod Identity Association.

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

### Complete Application Example

```yaml
# ServiceAccount with PIA annotations
apiVersion: v1
kind: ServiceAccount
metadata:
  name: s3-access-sa
  namespace: my-app
  annotations:
    pia-operator.eks.aws.com/role: "arn:aws:iam::123456789012:role/S3AccessRole"
---
# Deployment using the ServiceAccount
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      serviceAccountName: s3-access-sa
      containers:
      - name: my-app
        image: my-app:latest
        env:
        - name: AWS_REGION
          value: us-west-2
        # This container will automatically have access to the S3AccessRole
```

### Cross-Account Role Access

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cross-account-sa
  namespace: default
  annotations:
    # Base role in your account
    pia-operator.eks.aws.com/role: "arn:aws:iam::111111111111:role/BaseRole"
    # Role to assume in different account
    pia-operator.eks.aws.com/assume-role: "arn:aws:iam::222222222222:role/CrossAccountRole"
```

### Disabling Session Tags

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: no-session-tags-sa
  namespace: default
  annotations:
    pia-operator.eks.aws.com/role: "arn:aws:iam::123456789012:role/MyPodRole"
    # Disable session tags for this association
    pia-operator.eks.aws.com/tagging: "false"
```

## Monitoring and Metrics

The operator exposes Prometheus metrics on the configured metrics endpoint (default `:8080/metrics`):

### Available Metrics

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `pia_operator_pod_identity_association_errors_total` | Counter | Total number of errors when managing Pod Identity Associations | `operation` (create, update, delete) |
| `pia_operator_pod_identity_associations_managed` | Gauge | Number of Pod Identity Associations currently managed by the operator | - |

### Grafana Dashboard Example

```promql
# Error rate by operation
rate(pia_operator_pod_identity_association_errors_total[5m])

# Total managed associations
pia_operator_pod_identity_associations_managed

# Success rate
(
  rate(pia_operator_reconciliation_total[5m]) - 
  rate(pia_operator_pod_identity_association_errors_total[5m])
) / rate(pia_operator_reconciliation_total[5m]) * 100
```

### Health Checks

The operator provides health and readiness endpoints:

- **Liveness**: `http://localhost:8081/healthz`
- **Readiness**: `http://localhost:8081/readyz`

## Development

### Prerequisites

- Go 1.21+
- Docker
- kubectl
- Access to an EKS cluster
- [Taskfile](https://taskfile.dev) (Task runner)

### Installing Taskfile

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

# Deploy to Kubernetes
task deploy IMG=pia-operator:latest

# Uninstall
task undeploy
```

### Running Locally

To run the operator locally for development:

```bash
# Set required environment variables
export AWS_REGION=us-west-2
export CLUSTER_NAME=my-eks-cluster

# Run with development logging
go run main.go --dev-mode --cluster-name=$CLUSTER_NAME --aws-region=$AWS_REGION
```

## Troubleshooting

### Common Issues

1. **"Missing required annotation" errors**
   - Ensure `pia-operator.eks.aws.com/role` is set when using `pia-operator.eks.aws.com/assume-role`
   - Check that the annotation values are valid ARNs

2. **AWS permission errors**
   - Verify the operator's service account has the required AWS IAM permissions
   - Check that the service account is properly annotated with the IAM role ARN
   - Ensure the IAM role's trust policy allows the service account to assume the role

3. **"Role not found" errors**
   - Ensure the IAM roles specified in annotations exist
   - Verify the roles have proper trust policies for Pod Identity
   - Check that the role ARNs are correctly formatted

4. **Operator not processing ServiceAccounts**
   - Verify the operator is running: `kubectl get pods -n pia-operator-system`
   - Check operator logs for errors
   - Ensure cluster name is correctly configured

5. **Pod Identity Association creation fails**
   - Confirm EKS cluster has Pod Identity enabled
   - Verify AWS region configuration matches your cluster
   - Check AWS CloudTrail logs for detailed error information

### Debugging Commands

```bash
# Check operator status
kubectl get pods -n pia-operator-system

# View operator logs
kubectl logs -n pia-operator-system deployment/pia-operator-controller-manager -f

# Check ServiceAccount annotations
kubectl get serviceaccount <sa-name> -n <namespace> -o yaml

# View operator metrics
kubectl port-forward -n pia-operator-system deployment/pia-operator-controller-manager 8080:8080
curl http://localhost:8080/metrics

# Check health endpoints
kubectl port-forward -n pia-operator-system deployment/pia-operator-controller-manager 8081:8081
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
```

### Log Levels

Enable development mode for more verbose logging:

```bash
# Using Helm
helm upgrade pia-operator ./charts/pia-operator --set operator.devMode=true

# Or set via deployment args
--dev-mode
```

### AWS CloudTrail Events

Monitor these CloudTrail events to debug AWS API interactions:

- `CreatePodIdentityAssociation`
- `UpdatePodIdentityAssociation` 
- `DeletePodIdentityAssociation`
- `DescribePodIdentityAssociation`
- `ListPodIdentityAssociations`

## Security Considerations

- The operator runs with minimal privileges (non-root, dropped capabilities)
- RBAC is limited to ServiceAccount resources only
- AWS IAM permissions are scoped to Pod Identity Association operations
- Supports security contexts and pod security standards
- All container images should be scanned for vulnerabilities

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request
