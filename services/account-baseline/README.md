# Account Baseline Service

Deploys account baseline resources to a target AWS account. This service creates a consistent baseline configuration including cross-account access roles, password policies, region settings, and cost budgets.

## Features

- **PlatformReadOnlyRole** - Cross-account read-only access for auditing and monitoring
- **Password Policy** - Configurable IAM account password policy
- **Enabled Regions** - Control which AWS regions are enabled in the account
- **Monthly Budget** - Cost tracking with email notifications

## Created Resources

| Resource | Description |
|----------|-------------|
| `PlatformReadOnlyRole` | IAM role with ViewOnlyAccess for cross-account auditing |
| `AccountPasswordPolicy` | IAM password policy (when enabled) |
| `Region` | AWS region enablement (for each specified region) |
| `Budget` | Monthly cost budget with notifications (when enabled) |

## Configuration

### Required

| Parameter | Description |
|-----------|-------------|
| `trustedAccountId` | AWS account ID allowed to assume the readonly role |

### Optional - General

| Parameter | Default | Description |
|-----------|---------|-------------|
| `assumeRoleArn` | - | ARN of IAM role to assume for deployment |
| `aws:region` | `eu-north-1` | AWS region for the provider |

### Optional - Password Policy

| Parameter | Default | Description |
|-----------|---------|-------------|
| `passwordPolicy:enabled` | `false` | Enable IAM account password policy |
| `passwordPolicy:minLength` | `14` | Minimum password length |
| `passwordPolicy:maxAgeDays` | `0` | Maximum password age in days (0 = no expiry) |
| `passwordPolicy:reusePrevention` | `0` | Number of previous passwords to prevent reuse |

### Optional - Region Settings

| Parameter | Default | Description |
|-----------|---------|-------------|
| `enabledRegions` | - | Comma-separated list of AWS regions to enable |

### Optional - Budget

| Parameter | Default | Description |
|-----------|---------|-------------|
| `budget:enabled` | `false` | Enable monthly cost budget |
| `budget:limitAmount` | `100` | Budget limit amount |
| `budget:limitUnit` | `USD` | Budget limit currency |
| `budget:notificationEmail` | - | Email for budget notifications |
| `budget:threshold` | `80` | Threshold percentage for forecasted notification |

## Outputs

- `accountId` - Target account ID where resources are deployed
- `readonlyRoleArn` - ARN of the PlatformReadOnlyRole
- `readonlyRoleName` - Name of the readonly role
- `passwordPolicyEnabled` - Whether password policy is enabled
- `passwordPolicyMinLength` - Configured minimum password length
- `enabledRegions` - List of enabled regions
- `budgetName` - Name of the created budget
- `budgetLimitAmount` - Budget limit amount
- `budgetLimitUnit` - Budget limit currency

## Example Configurations

### Minimal (readonly role only)

```yaml
config:
  account-baseline-service:trustedAccountId: "111111111111"
```

### Full baseline with all features

```yaml
config:
  aws:region: eu-north-1
  account-baseline-service:trustedAccountId: "111111111111"
  account-baseline-service:assumeRoleArn: arn:aws:iam::TARGET_ACCOUNT_ID:role/OrganizationAccountAccessRole
  # Password policy
  account-baseline-service:passwordPolicy:enabled: true
  account-baseline-service:passwordPolicy:minLength: 14
  account-baseline-service:passwordPolicy:maxAgeDays: 90
  account-baseline-service:passwordPolicy:reusePrevention: 5
  # Enabled regions
  account-baseline-service:enabledRegions: "eu-west-1,eu-central-1,us-east-1"
  # Budget
  account-baseline-service:budget:enabled: true
  account-baseline-service:budget:limitAmount: "500"
  account-baseline-service:budget:limitUnit: USD
  account-baseline-service:budget:notificationEmail: alerts@example.com
  account-baseline-service:budget:threshold: 80
```

## Bootstrap Deployment

When deploying to a newly created account, use the `OrganizationAccountAccessRole` (or equivalent) to bootstrap the baseline:

```yaml
config:
  aws:region: eu-north-1
  account-baseline-service:trustedAccountId: "111111111111"
  account-baseline-service:assumeRoleArn: arn:aws:iam::TARGET_ACCOUNT_ID:role/OrganizationAccountAccessRole
```

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
    account-baseline-service:passwordPolicy:enabled: "true"
    account-baseline-service:budget:enabled: "true"
    account-baseline-service:budget:limitAmount: "200"
    account-baseline-service:budget:notificationEmail: platform-alerts@example.com
  destroyOnFinalize: false
```

## CLI Usage

```bash
cd services/account-baseline
pulumi stack init account-123456789012
pulumi config set trustedAccountId 111111111111
pulumi config set assumeRoleArn arn:aws:iam::123456789012:role/OrganizationAccountAccessRole
pulumi config set passwordPolicy:enabled true
pulumi config set budget:enabled true
pulumi config set budget:limitAmount 200
pulumi config set budget:notificationEmail alerts@example.com
pulumi up
```

## Trust Policy

The PlatformReadOnlyRole is created with the following trust policy:

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