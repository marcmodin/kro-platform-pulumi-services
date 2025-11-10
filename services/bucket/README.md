# Bucket Service

On-demand S3 bucket provisioning service for the Pulumi Kubernetes Operator.

## Features

- Automatic bucket naming with AWS account ID suffix for global uniqueness
- Public access always blocked (security by default)
- Server-side encryption enabled (AES256 or KMS)
- Optional versioning and lifecycle management
- Configurable AWS region deployment

## Configuration

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `bucket-service:bucketName` | Yes | - | Base bucket name (account ID appended automatically) |
| `aws:region` | No | `eu-north-1` | AWS region |
| `bucket-service:versioning` | No | `false` | Enable versioning |
| `bucket-service:encryption` | No | `AES256` | Encryption type (`AES256` or `aws:kms`) |
| `bucket-service:lifecycleEnabled` | No | `false` | Enable lifecycle rules (requires versioning) |
| `bucket-service:lifecycleDays` | No | `90` | Days before transition to IA storage |
| `bucket-service:expirationDays` | No | `0` | Days before expiring old versions (0=disabled) |

## Outputs

- `bucketName` - Full bucket name with account ID suffix
- `bucketArn` - Bucket ARN
- `bucketRegion` - AWS region
- `bucketUrl` - S3 URL format
- `bucketDomainName` - Bucket domain name
- `bucketRegionalDomainName` - Regional domain name

## Usage with Kubernetes Operator

```yaml
apiVersion: pulumi.com/v1
kind: Stack
metadata:
  name: my-app-bucket
spec:
  stack: organization/bucket-service/my-app
  projectRepo: https://github.com/your-org/kro-platform-pulumi-demo
  repoDir: services/bucket
  branch: main
  backend: s3://kro-platform-pulumi-state-{account-id}
  envRefs:
    PULUMI_CONFIG_PASSPHRASE:
      type: Literal
      literal:
        value: ""
  config:
    bucket-service:bucketName: my-app-data
    bucket-service:versioning: "true"
    bucket-service:encryption: AES256
  destroyOnFinalize: true
```

Apply the Stack:

```bash
kubectl apply -f stack.yaml
```

Check status:

```bash
kubectl get stack my-app-bucket
kubectl describe stack my-app-bucket
```

Get outputs:

```bash
kubectl get stack my-app-bucket -o jsonpath='{.status.outputs.bucketName}'
```

Delete (removes AWS resources if `destroyOnFinalize: true`):

```bash
kubectl delete stack my-app-bucket
```

## Naming Convention

Buckets are created as: `{bucketName}-{account-id}`

Example: `my-app-data` → `my-app-data-123456789012`