# Pulumi State Bucket Bootstrap

This stack creates and manages the S3 bucket used for storing Pulumi state files for other stacks in the platform.

## Features

- **S3 Bucket**: Named `kro-platform-pulumi-state-{account-id}` for global uniqueness
- **Versioning**: Enabled to maintain history of state file changes and enable rollback
- **Encryption**: AES256 server-side encryption with S3 bucket keys for cost optimization
- **Security**: Public access blocked on all levels
- **Region**: Deployed to eu-north-1

## Prerequisites

- AWS credentials configured
- Pulumi CLI installed
- Go 1.23.11 or later

## Deployment
### Option A: Self-Managed (Bootstrap state in its own S3 bucket)

This approach creates the bucket with a local backend, then migrates the bootstrap stack's state into the bucket it created.

#### 1. Install dependencies

```bash
cd bootstrap
go mod download
```

#### 2. Initialize with local backend

```bash
# Use local file-based backend for initial deployment
pulumi login --local

# Initialize the stack
pulumi stack init default
```

#### 3. Deploy the stack

```bash
pulumi up
```

Review and confirm to create the S3 bucket and related resources.

#### 4. Migrate bootstrap state to S3

Once the bucket is created, migrate the bootstrap stack's state to the S3 bucket:

```bash
# Set the pulumi config passphrase
export PULUMI_CONFIG_PASSPHRASE=""

# Get the bucket name from outputs
BUCKET_NAME=$(pulumi stack output bucketName)

# Login to the S3 backend
pulumi login s3://${BUCKET_NAME}

# Select the stack (this will migrate the state automatically)
pulumi stack select default

# Verify the migration worked
pulumi stack ls
```

#### 5. Verify the state is in S3

```bash
aws s3 ls s3://${BUCKET_NAME}/.pulumi/
```

You should see the `.pulumi/` directory structure with your bootstrap stack state.

### Option B: Separate Backend (Bootstrap state remains local/cloud)

Keep the bootstrap stack on a separate backend (simpler but requires maintaining two backends).

#### 1. Install dependencies

```bash
cd bootstrap
go mod download
```

#### 2. Choose your bootstrap backend

```bash
# Option 1: Use Pulumi Cloud (default)
pulumi login

# Option 2: Use local file backend
pulumi login --local
```

#### 3. Initialize the stack

```bash
pulumi stack init default
```

#### 4. Deploy the stack

```bash
pulumi up
```

#### 5. Note the outputs

After deployment, note the exported values:

```bash
pulumi stack output bucketName
pulumi stack output region
pulumi stack output stateBackendUrl
```

The bootstrap stack stays on your chosen backend, while other stacks can use the S3 bucket.

## Migrating Other Stacks to Use S3 Backend

Once the bootstrap stack is deployed, migrate your other stacks to use the S3 backend:

### Option 1: Using pulumi login

```bash
cd ../project
pulumi login s3://kro-platform-pulumi-state-{your-account-id}
```

### Option 2: Using environment variable

```bash
export PULUMI_BACKEND_URL=s3://kro-platform-pulumi-state-{your-account-id}
cd ../project
pulumi up
```

### Option 3: Per-stack configuration

For each stack that should use the S3 backend:

```bash
cd ../project
pulumi stack select your-stack-name
pulumi stack change-secrets-provider "awskms://alias/pulumi" # optional: use AWS KMS for secrets
```

## Backend Configuration

The bootstrap stack itself remains on the local/default backend. This is intentional - it's the foundational stack that creates the S3 bucket, so it cannot depend on that bucket for its own state.

## Maintenance

### Viewing state bucket contents

```bash
aws s3 ls s3://kro-platform-pulumi-state-{account-id}/
```

### Checking versioning status

```bash
aws s3api get-bucket-versioning --bucket kro-platform-pulumi-state-{account-id}
```

### Checking encryption configuration

```bash
aws s3api get-bucket-encryption --bucket kro-platform-pulumi-state-{account-id}
```

## Decommissioning

If you need to tear down the state bucket infrastructure, follow these steps carefully:

### Prerequisites

Before decommissioning the bootstrap stack:

1. **Migrate all stacks off the S3 backend**

   For each stack using the S3 backend:

   ```bash
   cd path/to/your/stack
   # Login to a different backend (e.g., local)
   pulumi login --local
   # or
   pulumi login

   # The stack state will be migrated automatically
   ```

2. **Verify no stacks are using the S3 backend**

   Check the bucket contents to ensure no active state files remain:

   ```bash
   aws s3 ls s3://kro-platform-pulumi-state-{account-id}/ --recursive
   ```

### Decommissioning Steps

#### Step 1: Empty the bucket

The S3 bucket must be empty before it can be destroyed. Remove all objects and versions:

```bash
# List and remove all object versions
aws s3api list-object-versions \
  --bucket kro-platform-pulumi-state-{account-id} \
  --output json \
  --query 'Versions[].{Key:Key,VersionId:VersionId}' | \
  jq -r '.[] | "--key \(.Key) --version-id \(.VersionId)"' | \
  xargs -I {} aws s3api delete-object --bucket kro-platform-pulumi-state-{account-id} {}

# List and remove all delete markers
aws s3api list-object-versions \
  --bucket kro-platform-pulumi-state-{account-id} \
  --output json \
  --query 'DeleteMarkers[].{Key:Key,VersionId:VersionId}' | \
  jq -r '.[] | "--key \(.Key) --version-id \(.VersionId)"' | \
  xargs -I {} aws s3api delete-object --bucket kro-platform-pulumi-state-{account-id} {}
```

Or use the AWS CLI to empty the bucket in one command:

```bash
aws s3 rm s3://kro-platform-pulumi-state-{account-id}/ --recursive
aws s3api delete-objects --bucket kro-platform-pulumi-state-{account-id} \
  --delete "$(aws s3api list-object-versions --bucket kro-platform-pulumi-state-{account-id} --output json | jq '{Objects: [.Versions[] | {Key:.Key, VersionId:.VersionId}], Quiet: false}')"
```

#### Step 2: Destroy the bootstrap stack

Once the bucket is empty:

```bash
cd bootstrap
pulumi destroy
```

Review the resources to be deleted and confirm.

#### Step 3: Remove the stack

After successful destruction:

```bash
pulumi stack rm default
# or whichever stack name you used
```

### Alternative: Force Deletion (Use with Caution)

If you need to force delete the bucket with contents (not recommended for production):

1. Temporarily disable versioning in the code or manually:
   ```bash
   aws s3api put-bucket-versioning \
     --bucket kro-platform-pulumi-state-{account-id} \
     --versioning-configuration Status=Suspended
   ```

2. Modify `main.go` to add `forceDestroy`:
   ```go
   bucket, err := s3.NewBucket(ctx, "pulumi-state-bucket", &s3.BucketArgs{
       Bucket:       pulumi.String(bucketName),
       ForceDestroy: pulumi.Bool(true), // Add this line
   }, pulumi.Provider(provider))
   ```

3. Apply the change:
   ```bash
   pulumi up
   ```

4. Then destroy:
   ```bash
   pulumi destroy
   ```

**Warning**: This will permanently delete all state files and their version history without recovery options.

## Troubleshooting

### Bucket name conflict

If you get a bucket name conflict, it means a bucket with that name already exists (either in your account or globally). You can modify the bucket name in `main.go:36`.

### Access denied errors

Ensure your AWS credentials have sufficient permissions to:
- Create and manage S3 buckets
- Configure bucket versioning, encryption, and public access blocks
- Read AWS caller identity (for account ID)

### State file conflicts

If multiple stacks write to the same state file path in the bucket, you may encounter conflicts. Ensure each stack has a unique name or use stack-specific prefixes.

## Resources Created

- `pulumi-state-bucket`: S3 bucket for state storage
- `pulumi-state-versioning`: Versioning configuration
- `pulumi-state-encryption`: Encryption configuration
- `pulumi-state-public-access-block`: Public access block settings