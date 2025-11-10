package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Load configuration
		cfg := config.New(ctx, "")
		bucketName := cfg.Require("bucketName")
		versioning := cfg.GetBool("versioning")
		encryption := cfg.Get("encryption")
		if encryption == "" {
			encryption = "AES256"
		}
		lifecycleEnabled := cfg.GetBool("lifecycleEnabled")
		lifecycleDays := cfg.GetInt("lifecycleDays")
		if lifecycleDays == 0 {
			lifecycleDays = 90
		}
		expirationDays := cfg.GetInt("expirationDays")

		// Get AWS region from config
		awsCfg := config.New(ctx, "aws")
		region := awsCfg.Get("region")
		if region == "" {
			region = "eu-north-1"
		}

		// Create AWS provider
		provider, err := aws.NewProvider(ctx, "aws-provider", &aws.ProviderArgs{
			Region: pulumi.String(region),
			DefaultTags: &aws.ProviderDefaultTagsArgs{
				Tags: pulumi.StringMap{
					"created_with": pulumi.String("pulumi"),
					"service":      pulumi.String("bucket-service"),
					"managed_by":   pulumi.String("pulumi-kubernetes-operator"),
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

		// Create bucket name with account ID suffix for global uniqueness
		fullBucketName := fmt.Sprintf("%s-%s", bucketName, current.AccountId)

		// Create S3 bucket
		bucket, err := s3.NewBucket(ctx, "bucket", &s3.BucketArgs{
			Bucket: pulumi.String(fullBucketName),
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// Configure versioning if enabled
		if versioning {
			_, err = s3.NewBucketVersioningV2(ctx, "bucket-versioning", &s3.BucketVersioningV2Args{
				Bucket: bucket.ID(),
				VersioningConfiguration: &s3.BucketVersioningV2VersioningConfigurationArgs{
					Status: pulumi.String("Enabled"),
				},
			}, pulumi.Provider(provider))
			if err != nil {
				return err
			}
		}

		// Configure encryption
		var encryptionRule *s3.BucketServerSideEncryptionConfigurationV2RuleArgs
		if encryption == "AES256" {
			encryptionRule = &s3.BucketServerSideEncryptionConfigurationV2RuleArgs{
				ApplyServerSideEncryptionByDefault: &s3.BucketServerSideEncryptionConfigurationV2RuleApplyServerSideEncryptionByDefaultArgs{
					SseAlgorithm: pulumi.String("AES256"),
				},
				BucketKeyEnabled: pulumi.Bool(true),
			}
		} else if encryption == "aws:kms" {
			encryptionRule = &s3.BucketServerSideEncryptionConfigurationV2RuleArgs{
				ApplyServerSideEncryptionByDefault: &s3.BucketServerSideEncryptionConfigurationV2RuleApplyServerSideEncryptionByDefaultArgs{
					SseAlgorithm: pulumi.String("aws:kms"),
				},
				BucketKeyEnabled: pulumi.Bool(true),
			}
		}

		if encryptionRule != nil {
			_, err = s3.NewBucketServerSideEncryptionConfigurationV2(ctx, "bucket-encryption", &s3.BucketServerSideEncryptionConfigurationV2Args{
				Bucket: bucket.ID(),
				Rules: s3.BucketServerSideEncryptionConfigurationV2RuleArray{
					encryptionRule,
				},
			}, pulumi.Provider(provider))
			if err != nil {
				return err
			}
		}

		// Configure lifecycle rules if enabled
		if lifecycleEnabled && versioning {
			var rules s3.BucketLifecycleConfigurationV2RuleArray

			// Transition rule
			if lifecycleDays > 0 {
				rules = append(rules, &s3.BucketLifecycleConfigurationV2RuleArgs{
					Id:     pulumi.String("transition-old-versions"),
					Status: pulumi.String("Enabled"),
					NoncurrentVersionTransitions: s3.BucketLifecycleConfigurationV2RuleNoncurrentVersionTransitionArray{
						&s3.BucketLifecycleConfigurationV2RuleNoncurrentVersionTransitionArgs{
							NoncurrentDays: pulumi.Int(lifecycleDays),
							StorageClass:   pulumi.String("STANDARD_IA"),
						},
					},
				})
			}

			// Expiration rule
			if expirationDays > 0 {
				rules = append(rules, &s3.BucketLifecycleConfigurationV2RuleArgs{
					Id:     pulumi.String("expire-old-versions"),
					Status: pulumi.String("Enabled"),
					NoncurrentVersionExpiration: &s3.BucketLifecycleConfigurationV2RuleNoncurrentVersionExpirationArgs{
						NoncurrentDays: pulumi.Int(expirationDays),
					},
				})
			}

			if len(rules) > 0 {
				_, err = s3.NewBucketLifecycleConfigurationV2(ctx, "bucket-lifecycle", &s3.BucketLifecycleConfigurationV2Args{
					Bucket: bucket.ID(),
					Rules:  rules,
				}, pulumi.Provider(provider))
				if err != nil {
					return err
				}
			}
		}

		// Always block public access (security best practice)
		_, err = s3.NewBucketPublicAccessBlock(ctx, "bucket-public-access-block", &s3.BucketPublicAccessBlockArgs{
			Bucket:                bucket.ID(),
			BlockPublicAcls:       pulumi.Bool(true),
			BlockPublicPolicy:     pulumi.Bool(true),
			IgnorePublicAcls:      pulumi.Bool(true),
			RestrictPublicBuckets: pulumi.Bool(true),
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// Export bucket information
		ctx.Export("bucketName", bucket.ID())
		ctx.Export("bucketArn", bucket.Arn)
		ctx.Export("bucketRegion", pulumi.String(region))
		ctx.Export("bucketUrl", pulumi.Sprintf("s3://%s", bucket.ID()))
		ctx.Export("bucketDomainName", bucket.BucketDomainName)
		ctx.Export("bucketRegionalDomainName", bucket.BucketRegionalDomainName)

		// Export configuration summary
		ctx.Export("config", pulumi.Map{
			"versioning":       pulumi.Bool(versioning),
			"encryption":       pulumi.String(encryption),
			"lifecycleEnabled": pulumi.Bool(lifecycleEnabled),
			"publicAccess":     pulumi.Bool(false),
		})

		return nil
	})
}
