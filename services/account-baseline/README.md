# Account Baseline Service

Deploys platform-assumable IAM roles to a target AWS account. This service creates a consistent baseline of cross-account access roles that allow the platform account to manage resources in member accounts.

## Features

- Creates two IAM roles with fixed names for consistency across all accounts
- **PlatformAdminRole** - Full administrator access
- **PlatformReadOnlyRole** - View-only access
- Cross-account trust policy to the platform account
- Support for bootstrap deployment via OrganizationAccountAccessRole

## Created Resources

| Role Name | Attached Policy | Description |
|-----------|-----------------|-------------|
| `PlatformAdminRole` | `AdministratorAccess` | Full access for platform operations |
| `PlatformReadOnlyRole` | `ViewOnlyAccess` | Read-only access for auditing and monitoring |

## Configuration

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `account-baseline-service:trustedAccountId` | Yes | - | AWS account ID allowed to assume these roles |
| `account-baseline-service:assumeRoleArn` | No | - | ARN of IAM role to assume for deployment |
| `aws:region` | No | `eu-north-1` | AWS region |

## Outputs

- `accountId` - Target account ID where roles are deployed
- `adminRoleArn` - ARN of the PlatformAdminRole
- `adminRoleName` - Name of the admin role
- `readonlyRoleArn` - ARN of the PlatformReadOnlyRole
- `readonlyRoleName` - Name of the readonly role

## Bootstrap Deployment

When deploying to a newly created account, use the `OrganizationAccountAccessRole` (or equivalent) to bootstrap the baseline:

```yaml
config:
  aws:region: eu-north-1
  account-baseline-service:trustedAccountId: "111111111111"
  account-baseline-service:assumeRoleArn: arn:aws:iam::TARGET_ACCOUNT_ID:role/OrganizationAccountAccessRole
```

After the baseline is deployed, subsequent deployments can use `PlatformAdminRole`.

## Usage with Kubernetes Operator

```yaml
apiVersion: pulumi.com/v1
kind: Stack
metadata:
  name: account-baseline-123456789012
spec:
  stack: organization/account-baseline-service/account-123456789012
  projectRepo: https://github.com/marcmodin/kro-platform-pulumi-services
  repoDir: services/account-baseline
  branch: main
  backend: s3://pulumi-state-bucket
  envRefs:
    PULUMI_CONFIG_PASSPHRASE:
      type: Literal
      literal:
        value: ""
  config:
    account-baseline-service:trustedAccountId: "111111111111"
    account-baseline-service:assumeRoleArn: arn:aws:iam::123456789012:role/OrganizationAccountAccessRole
  destroyOnFinalize: false
```

## CLI Usage

```bash
cd services/account-baseline
pulumi stack init account-123456789012
pulumi config set trustedAccountId 111111111111
pulumi config set assumeRoleArn arn:aws:iam::123456789012:role/OrganizationAccountAccessRole
pulumi up
```

## Trust Policy

Both roles are created with the following trust policy, allowing the specified platform account to assume them:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::TRUSTED_ACCOUNT_ID:root"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```