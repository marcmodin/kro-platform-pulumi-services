package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// eu-north-1 provider
		provider, err := aws.NewProvider(ctx, "eu-north-1", &aws.ProviderArgs{
			Region: pulumi.StringPtr("eu-north-1"),
			DefaultTags: &aws.ProviderDefaultTagsArgs{
				Tags: pulumi.StringMap{
					"created_with": pulumi.String("pulumi"),
					"purpose":      pulumi.String("pulumi-state"),
				},
			},
		})

		if err != nil {
			return err
		}

		// Get current AWS account ID
		current, err := aws.GetCallerIdentity(ctx, &aws.GetCallerIdentityArgs{}, nil, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// Create S3 bucket for Pulumi state
		bucketName := fmt.Sprintf("kro-platform-pulumi-state-%s", current.AccountId)
		bucket, err := s3.NewBucket(ctx, "pulumi-state-bucket", &s3.BucketArgs{
			Bucket: pulumi.String(bucketName),
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// Enable versioning for state file history
		_, err = s3.NewBucketVersioningV2(ctx, "pulumi-state-versioning", &s3.BucketVersioningV2Args{
			Bucket: bucket.ID(),
			VersioningConfiguration: &s3.BucketVersioningV2VersioningConfigurationArgs{
				Status: pulumi.String("Enabled"),
			},
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// Enable server-side encryption with AES256
		_, err = s3.NewBucketServerSideEncryptionConfigurationV2(ctx, "pulumi-state-encryption", &s3.BucketServerSideEncryptionConfigurationV2Args{
			Bucket: bucket.ID(),
			Rules: s3.BucketServerSideEncryptionConfigurationV2RuleArray{
				&s3.BucketServerSideEncryptionConfigurationV2RuleArgs{
					ApplyServerSideEncryptionByDefault: &s3.BucketServerSideEncryptionConfigurationV2RuleApplyServerSideEncryptionByDefaultArgs{
						SseAlgorithm: pulumi.String("AES256"),
					},
					BucketKeyEnabled: pulumi.Bool(true),
				},
			},
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// Block public access
		_, err = s3.NewBucketPublicAccessBlock(ctx, "pulumi-state-public-access-block", &s3.BucketPublicAccessBlockArgs{
			Bucket:                bucket.ID(),
			BlockPublicAcls:       pulumi.Bool(true),
			BlockPublicPolicy:     pulumi.Bool(true),
			IgnorePublicAcls:      pulumi.Bool(true),
			RestrictPublicBuckets: pulumi.Bool(true),
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// Export the bucket name and region
		ctx.Export("bucketName", bucket.ID())
		ctx.Export("region", provider.Region)
		ctx.Export("stateBackendUrl", pulumi.Sprintf("s3://%s", bucket.ID()))

		return nil
	})
}