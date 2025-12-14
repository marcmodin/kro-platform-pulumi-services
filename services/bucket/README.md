# Bucket Service

On-demand S3 bucket provisioning service for the Pulumi Kubernetes Operator.

## Features

- Automatic bucket naming with AWS account ID suffix for global uniqueness
- Public access always blocked (security by default)
- Server-side encryption enabled (AES256 or KMS)
- Optional versioning and lifecycle management
- Support for cross-account assume role when running from a different account
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
| `bucket-service:assumeRoleArn` | No | - | ARN of IAM role to assume in the target account |

## Outputs

- `bucketName` - Full bucket name with account ID suffix
- `bucketArn` - Bucket ARN
- `bucketRegion` - AWS region
- `bucketUrl` - S3 URL format
- `bucketDomainName` - Bucket domain name
- `bucketRegionalDomainName` - Regional domain name

## Cross-Account Assume Role

When the Pulumi stack runs from a different AWS account than where you want to create the bucket, you must configure `assumeRoleArn` to assume a role with permissions to create S3 resources.

### Prerequisites

1. An IAM role in the target account with:
   - Trust policy allowing the assuming account to assume it
   - Permissions for `s3:CreateBucket`, `s3:PutBucketEncryption`, `s3:PutBucketVersioning`, `s3:PutBucketPublicAccessBlock`, `s3:PutLifecycleConfiguration`, etc.

2. The assuming account credentials must have permission to assume this role

### Example Trust Policy (in target account)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::ASSUMING_ACCOUNT_ID:root"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

### Stack Configuration with Assume Role

```yaml
config:
  aws:region: eu-north-1
  bucket-service:bucketName: my-app-data
  bucket-service:versioning: "true"
  bucket-service:assumeRoleArn: arn:aws:iam::TARGET_ACCOUNT_ID:role/S3Administrator
```

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
    bucket-service:assumeRoleArn: arn:aws:iam::TARGET_ACCOUNT_ID:role/S3Administrator
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

## CLI Usage

```bash
cd services/bucket
pulumi stack init my-bucket
pulumi up
```

## Naming Convention

Buckets are created as: `{bucketName}-{account-id}`

Example: `my-app-data` → `my-app-data-123456789012`
