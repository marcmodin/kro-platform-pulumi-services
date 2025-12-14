# Account Service

On-demand AWS account provisioning service for the Pulumi Kubernetes Operator using AWS Organizations.

## Features

- Creates AWS accounts within an AWS Organization
- Configurable cross-account IAM role for access
- Optional placement in specific Organizational Units (OUs)
- Support for cross-account assume role when running from a different account
- Automatic account suspension workflow via delete hooks

## Configuration

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `account-service:accountName` | Yes | - | Name of the AWS account to create |
| `account-service:email` | Yes | - | Email address for the root user of the new account |
| `aws:region` | No | `eu-north-1` | AWS region for Organizations API |
| `account-service:roleName` | No | `OrganizationAccountAccessRole` | IAM role name for cross-account access |
| `account-service:parentId` | No | - | Parent OU or root ID to place the account in |
| `account-service:assumeRoleArn` | No | - | ARN of IAM role to assume in the organization management account |
| `account-service:suspendedOuId` | No | - | OU ID to move the account to when the stack is destroyed |

## Outputs

- `accountId` - The AWS account ID of the created account
- `accountArn` - The ARN of the created account
- `accountName` - The friendly name of the account
- `accountEmail` - The email associated with the account
- `accountState` - The current state of the account
- `roleName` - The IAM role name for cross-account access

## Account Lifecycle Management

This service uses Pulumi resource hooks to manage account lifecycle. Instead of closing accounts on deletion (which causes timeout issues and can be impacted by qouta limits), accounts are moved to a suspended OU.

### How it works

1. **On create**: Account is created in the specified `parentId` OU
2. **On destroy**: If `suspendedOuId` is configured, the account is moved to the suspended OU before being removed from Pulumi state
3. **Retention**: Accounts are retained in AWS (`retainOnDelete: true`) - they are not closed automatically and you must handle closure separately
4. **External process**: A separate process outside of Pulumi that you manage, handles the actual account closure from the suspended OU

### Why this approach?

- AWS account closure is asynchronous and can cause Pulumi timeouts
- Accounts enter a 90-day suspension period before permanent deletion
- Moving to a suspended OU allows for review before closure
- Avoids AWS quota limits on account closures per 30 day period

## Cross-Account Assume Role

When the Pulumi stack runs from a different AWS account than the organization management account, you must configure `assumeRoleArn` to assume a role with permissions to manage accounts.

### Prerequisites

1. An IAM role in the organization management account with:
   - Trust policy allowing the platform account to assume it
   - Permissions for `organizations:CreateAccount`, `organizations:DescribeAccount`, `organizations:MoveAccount`, `organizations:ListParents`, etc.

2. The assuming account credentials must have permission to assume this role

### Example Trust Policy (in organization management account)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::PLATFORM_ACCOUNT_ID:root"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

### Stack Configuration

```yaml
config:
  aws:region: eu-north-1
  account-service:accountName: my-new-account
  account-service:email: my-account@example.com
  account-service:parentId: ou-xxxx-xxxxxxxx
  account-service:assumeRoleArn: arn:aws:iam::ORG_ACCOUNT_ID:role/OrganizationAdministrator
  account-service:suspendedOuId: ou-xxxx-suspended
```

## Usage with Kubernetes Operator

```yaml
apiVersion: pulumi.com/v1
kind: Stack
metadata:
  name: sandbox-account
spec:
  stack: organization/account-service/sandbox
  projectRepo: https://github.com/your-org/kro-platform-pulumi-demo
  repoDir: services/account
  branch: main
  backend: s3://kro-platform-pulumi-state-{account-id}
  envRefs:
    PULUMI_CONFIG_PASSPHRASE:
      type: Literal
      literal:
        value: ""
  config:
    account-service:accountName: sandbox-01
    account-service:email: sandbox-01@example.com
    account-service:parentId: ou-xxxx-xxxxxxxx
    account-service:assumeRoleArn: arn:aws:iam::ORG_ACCOUNT_ID:role/OrganizationAdministrator
    account-service:suspendedOuId: ou-xxxx-suspended
  destroyOnFinalize: true
```

Apply the Stack:

```bash
kubectl apply -f stack.yaml
```

Check status:

```bash
kubectl get stack sandbox-account
kubectl describe stack sandbox-account
```

Get outputs:

```bash
kubectl get stack sandbox-account -o jsonpath='{.status.outputs.accountId}'
```

Delete (moves account to suspended OU):

```bash
kubectl delete stack sandbox-account
```

## CLI Usage

```bash
cd services/account
pulumi stack init sandbox-02
pulumi up
```

To destroy (moves account to suspended OU if configured):

```bash
pulumi destroy --run-program
```

## Important Notes

- Each AWS account requires a unique email address
- Account creation can take several minutes to complete
- Accounts are **not closed** on stack deletion - they are moved to the suspended OU or retained in place
- A separate external process should handle account closure from the suspended OU
- The `suspendedOuId` must exist before destroying stacks that use it
