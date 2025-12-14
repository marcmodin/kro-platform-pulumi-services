# IAM Role Service

On-demand IAM role and policy provisioning service for the Pulumi Kubernetes Operator.

## Features

- Simple role creation assumable by account root
- **Preconfigured policy templates** - no need to know IAM policy syntax
- Multiple policies can be attached to a single role
- Support for S3 and DynamoDB services (easily expandable)
- Automatic policy naming and management
- Support for cross-account assume role when running from a different account
- Configurable AWS region deployment

## Available Policy Templates

| Policy Type | Description | Resources Required |
|-------------|-------------|-------------------|
| `s3-full-access` | Full access to S3 buckets (read, write, delete) | S3 bucket ARNs |
| `s3-read-only` | Read-only access to S3 buckets | S3 bucket ARNs |
| `s3-write-only` | Write-only access to S3 buckets | S3 bucket ARNs |
| `dynamodb-full-access` | Full access to DynamoDB tables | DynamoDB table ARNs |
| `dynamodb-read-only` | Read-only access to DynamoDB tables | DynamoDB table ARNs |

## Configuration

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `iam-role-service:roleName` | Yes | - | Name of the IAM role to create |
| `iam-role-service:description` | No | Auto-generated | Description for the IAM role |
| `iam-role-service:policies` | Yes | - | Policy configurations (see format below) |
| `aws:region` | No | `eu-north-1` | AWS region |
| `iam-role-service:assumeRoleArn` | No | - | ARN of IAM role to assume in the target account |

### Policy Configuration Format

Policies are specified in the format:
```
policyType:resource1,resource2;policyType2:resource3,resource4
```

- Multiple policies are separated by semicolons (`;`)
- Each policy specifies a type and one or more resources separated by colons (`:`)
- Multiple resources for the same policy are comma-separated (`,`)

**Examples:**

Single policy:
```
s3-full-access:arn:aws:s3:::my-bucket
```

Multiple resources for one policy:
```
s3-read-only:arn:aws:s3:::bucket1,arn:aws:s3:::bucket2,arn:aws:s3:::bucket3
```

Multiple policies:
```
s3-full-access:arn:aws:s3:::my-app-data;dynamodb-read-only:arn:aws:dynamodb:eu-north-1:123456789012:table/my-table
```

## Outputs

- `roleArn` - IAM role ARN
- `roleName` - IAM role name
- `roleId` - IAM role unique ID
- `accountId` - AWS account ID
- `policyCount` - Number of policies attached
- `policyNames` - Array of policy names
- `policyArns` - Array of policy ARNs
- `availablePolicyTemplates` - List of available policy types

## Cross-Account Assume Role

When the Pulumi stack runs from a different AWS account than where you want to create the IAM role, you must configure `assumeRoleArn` to assume a role with permissions to create IAM resources.

### Prerequisites

1. An IAM role in the target account with:
   - Trust policy allowing the assuming account to assume it
   - Permissions for `iam:CreateRole`, `iam:CreatePolicy`, `iam:AttachRolePolicy`, `iam:DeleteRole`, `iam:DeletePolicy`, `iam:DetachRolePolicy`, etc.

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
  iam-role-service:roleName: my-app-role
  iam-role-service:policies: "s3-full-access:arn:aws:s3:::my-bucket"
  iam-role-service:assumeRoleArn: arn:aws:iam::TARGET_ACCOUNT_ID:role/IAMAdministrator
```

## Usage with Kubernetes Operator

### Example 1: S3 Full Access Role

Create a role with full access to a single S3 bucket:

```yaml
apiVersion: pulumi.com/v1
kind: Stack
metadata:
  name: app-s3-role
spec:
  stack: organization/iam-role-service/app-s3
  projectRepo: https://github.com/your-org/kro-platform-pulumi-demo
  repoDir: services/iam-role
  branch: main
  backend: s3://kro-platform-pulumi-state-{account-id}
  envRefs:
    PULUMI_CONFIG_PASSPHRASE:
      type: Literal
      literal:
        value: ""
  config:
    iam-role-service:roleName: app-s3-access-role
    iam-role-service:description: Role for application to access S3 bucket
    iam-role-service:policies: "s3-full-access:arn:aws:s3:::my-app-data-123456789012"
  destroyOnFinalize: true
```

### Example 2: Multiple S3 Buckets with Read-Only Access

```yaml
apiVersion: pulumi.com/v1
kind: Stack
metadata:
  name: analytics-s3-role
spec:
  stack: organization/iam-role-service/analytics
  projectRepo: https://github.com/your-org/kro-platform-pulumi-demo
  repoDir: services/iam-role
  branch: main
  backend: s3://kro-platform-pulumi-state-{account-id}
  envRefs:
    PULUMI_CONFIG_PASSPHRASE:
      type: Literal
      literal:
        value: ""
  config:
    iam-role-service:roleName: analytics-readonly-role
    iam-role-service:policies: "s3-read-only:arn:aws:s3:::logs-bucket,arn:aws:s3:::metrics-bucket,arn:aws:s3:::events-bucket"
  destroyOnFinalize: true
```

### Example 3: Mixed Policies (S3 + DynamoDB)

Create a role with full S3 access and read-only DynamoDB access:

```yaml
apiVersion: pulumi.com/v1
kind: Stack
metadata:
  name: app-data-role
spec:
  stack: organization/iam-role-service/app-data
  projectRepo: https://github.com/your-org/kro-platform-pulumi-demo
  repoDir: services/iam-role
  branch: main
  backend: s3://kro-platform-pulumi-state-{account-id}
  envRefs:
    PULUMI_CONFIG_PASSPHRASE:
      type: Literal
      literal:
        value: ""
  config:
    iam-role-service:roleName: app-data-access-role
    iam-role-service:description: Application data access role for S3 and DynamoDB
    iam-role-service:policies: "s3-full-access:arn:aws:s3:::app-uploads;dynamodb-read-only:arn:aws:dynamodb:eu-north-1:123456789012:table/UserData"
  destroyOnFinalize: true
```

### Example 4: Separate Read/Write S3 Access

Create a role with read access to one bucket and write access to another:

```yaml
apiVersion: pulumi.com/v1
kind: Stack
metadata:
  name: etl-s3-role
spec:
  stack: organization/iam-role-service/etl
  projectRepo: https://github.com/your-org/kro-platform-pulumi-demo
  repoDir: services/iam-role
  branch: main
  backend: s3://kro-platform-pulumi-state-{account-id}
  envRefs:
    PULUMI_CONFIG_PASSPHRASE:
      type: Literal
      literal:
        value: ""
  config:
    iam-role-service:roleName: etl-pipeline-role
    iam-role-service:description: ETL pipeline with read source and write destination
    iam-role-service:policies: "s3-read-only:arn:aws:s3:::source-data;s3-write-only:arn:aws:s3:::processed-data"
  destroyOnFinalize: true
```

## Managing the Stack

Apply the Stack:
```bash
kubectl apply -f stack.yaml
```

Check status:
```bash
kubectl get stack app-s3-role
kubectl describe stack app-s3-role
```

Get role ARN:
```bash
kubectl get stack app-s3-role -o jsonpath='{.status.outputs.roleArn}'
```

Get all policy ARNs:
```bash
kubectl get stack app-s3-role -o jsonpath='{.status.outputs.policyArns}'
```

Delete (removes AWS resources if `destroyOnFinalize: true`):
```bash
kubectl delete stack app-s3-role
```

## Assuming the Role

Once created, you can assume the role from the same AWS account:

```bash
aws sts assume-role \
  --role-arn "arn:aws:iam::123456789012:role/app-s3-access-role" \
  --role-session-name "my-session"
```

Or configure it in your application using AWS SDK:

```python
# Python example
import boto3

sts_client = boto3.client('sts')
assumed_role = sts_client.assume_role(
    RoleArn='arn:aws:iam::123456789012:role/app-s3-access-role',
    RoleSessionName='my-app-session'
)

credentials = assumed_role['Credentials']
s3_client = boto3.client(
    's3',
    aws_access_key_id=credentials['AccessKeyId'],
    aws_secret_access_key=credentials['SecretAccessKey'],
    aws_session_token=credentials['SessionToken']
)
```

## Extending with New Policy Templates

To add new policy types, edit `main.go` and add entries to the `policyTemplates` map:

```go
"sqs-full-access": {
    Name:        "SQS Full Access",
    Description: "Full access to specified SQS queues",
    Generator: func(resources []string) map[string]interface{} {
        return map[string]interface{}{
            "Version": "2012-10-17",
            "Statement": []map[string]interface{}{
                {
                    "Effect": "Allow",
                    "Action": []string{
                        "sqs:SendMessage",
                        "sqs:ReceiveMessage",
                        "sqs:DeleteMessage",
                        "sqs:GetQueueAttributes",
                    },
                    "Resource": resources,
                },
            },
        }
    },
},
```

Then users can use it like:
```
sqs-full-access:arn:aws:sqs:eu-north-1:123456789012:my-queue
```

## Security Considerations

- **Least Privilege**: Always use the most restrictive policy template that meets your needs
- **Resource Specificity**: Specify exact resource ARNs instead of wildcards when possible
- **Role Trust**: By default, roles are assumable by the account root. For production use, consider restricting the trust policy to specific principals
- **Regular Audits**: Regularly review and audit role usage and permissions

## Notes

- Role can be assumed by any principal in the same AWS account (root trust policy)
- All resources are automatically tagged with `service: iam-role-service`
- Policy names are auto-generated as `{roleName}-{policyType}-{index}`
- S3 bucket ARNs should be in the format: `arn:aws:s3:::bucket-name`
- DynamoDB table ARNs should be in the format: `arn:aws:dynamodb:region:account-id:table/table-name`