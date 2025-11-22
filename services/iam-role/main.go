package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// PolicyTemplate defines a preconfigured policy that users can select
type PolicyTemplate struct {
	Name        string
	Description string
	Generator   func(resources []string) map[string]interface{}
}

// Available policy templates
var policyTemplates = map[string]PolicyTemplate{
	"s3-full-access": {
		Name:        "S3 Full Access",
		Description: "Full access to specified S3 buckets",
		Generator: func(resources []string) map[string]interface{} {
			return map[string]interface{}{
				"Version": "2012-10-17",
				"Statement": []map[string]interface{}{
					{
						"Effect": "Allow",
						"Action": []string{
							"s3:ListBucket",
							"s3:GetBucketLocation",
							"s3:GetBucketVersioning",
							"s3:ListBucketVersions",
							"s3:ListBucketMultipartUploads",
						},
						"Resource": resources,
					},
					{
						"Effect": "Allow",
						"Action": []string{
							"s3:GetObject",
							"s3:GetObjectVersion",
							"s3:PutObject",
							"s3:PutObjectAcl",
							"s3:DeleteObject",
							"s3:DeleteObjectVersion",
							"s3:RestoreObject",
							"s3:AbortMultipartUpload",
							"s3:ListMultipartUploadParts",
						},
						"Resource": appendSuffix(resources, "/*"),
					},
				},
			}
		},
	},
	"s3-read-only": {
		Name:        "S3 Read Only",
		Description: "Read-only access to specified S3 buckets",
		Generator: func(resources []string) map[string]interface{} {
			return map[string]interface{}{
				"Version": "2012-10-17",
				"Statement": []map[string]interface{}{
					{
						"Effect": "Allow",
						"Action": []string{
							"s3:ListBucket",
							"s3:GetBucketLocation",
							"s3:GetBucketVersioning",
							"s3:ListBucketVersions",
						},
						"Resource": resources,
					},
					{
						"Effect": "Allow",
						"Action": []string{
							"s3:GetObject",
							"s3:GetObjectVersion",
						},
						"Resource": appendSuffix(resources, "/*"),
					},
				},
			}
		},
	},
	"s3-write-only": {
		Name:        "S3 Write Only",
		Description: "Write-only access to specified S3 buckets (list, put, delete)",
		Generator: func(resources []string) map[string]interface{} {
			return map[string]interface{}{
				"Version": "2012-10-17",
				"Statement": []map[string]interface{}{
					{
						"Effect": "Allow",
						"Action": []string{
							"s3:ListBucket",
							"s3:GetBucketLocation",
							"s3:ListBucketMultipartUploads",
						},
						"Resource": resources,
					},
					{
						"Effect": "Allow",
						"Action": []string{
							"s3:PutObject",
							"s3:PutObjectAcl",
							"s3:DeleteObject",
							"s3:AbortMultipartUpload",
							"s3:ListMultipartUploadParts",
						},
						"Resource": appendSuffix(resources, "/*"),
					},
				},
			}
		},
	},
	"dynamodb-full-access": {
		Name:        "DynamoDB Full Access",
		Description: "Full access to specified DynamoDB tables",
		Generator: func(resources []string) map[string]interface{} {
			return map[string]interface{}{
				"Version": "2012-10-17",
				"Statement": []map[string]interface{}{
					{
						"Effect": "Allow",
						"Action": []string{
							"dynamodb:BatchGetItem",
							"dynamodb:BatchWriteItem",
							"dynamodb:DeleteItem",
							"dynamodb:GetItem",
							"dynamodb:PutItem",
							"dynamodb:Query",
							"dynamodb:Scan",
							"dynamodb:UpdateItem",
							"dynamodb:DescribeTable",
							"dynamodb:GetRecords",
						},
						"Resource": resources,
					},
				},
			}
		},
	},
	"dynamodb-read-only": {
		Name:        "DynamoDB Read Only",
		Description: "Read-only access to specified DynamoDB tables",
		Generator: func(resources []string) map[string]interface{} {
			return map[string]interface{}{
				"Version": "2012-10-17",
				"Statement": []map[string]interface{}{
					{
						"Effect": "Allow",
						"Action": []string{
							"dynamodb:BatchGetItem",
							"dynamodb:GetItem",
							"dynamodb:Query",
							"dynamodb:Scan",
							"dynamodb:DescribeTable",
							"dynamodb:GetRecords",
						},
						"Resource": resources,
					},
				},
			}
		},
	},
}

// Helper function to append suffix to resource ARNs
func appendSuffix(resources []string, suffix string) []string {
	result := make([]string, len(resources))
	for i, r := range resources {
		result[i] = r + suffix
	}
	return result
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Load configuration
		cfg := config.New(ctx, "")
		roleName := cfg.Require("roleName")
		description := cfg.Get("description")
		if description == "" {
			description = fmt.Sprintf("IAM role %s managed by Pulumi", roleName)
		}

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
					"service":      pulumi.String("iam-role-service"),
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

		// Create assume role policy document (allows account root to assume the role)
		assumeRolePolicy := map[string]interface{}{
			"Version": "2012-10-17",
			"Statement": []map[string]interface{}{
				{
					"Effect": "Allow",
					"Principal": map[string]interface{}{
						"AWS": fmt.Sprintf("arn:aws:iam::%s:root", current.AccountId),
					},
					"Action": "sts:AssumeRole",
				},
			},
		}

		assumeRolePolicyJSON, err := json.Marshal(assumeRolePolicy)
		if err != nil {
			return err
		}

		// Create IAM role
		role, err := iam.NewRole(ctx, "role", &iam.RoleArgs{
			Name:             pulumi.String(roleName),
			Description:      pulumi.String(description),
			AssumeRolePolicy: pulumi.String(string(assumeRolePolicyJSON)),
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// Parse policy configurations
		// Format: policyType1:resource1,resource2;policyType2:resource3
		policiesConfig := cfg.Get("policies")
		if policiesConfig == "" {
			return fmt.Errorf("no policies configured - please specify at least one policy")
		}

		// Track created policies for outputs
		policyNames := []string{}
		policyArns := []pulumi.StringOutput{}

		// Parse and create policies
		policyConfigs := strings.Split(policiesConfig, ";")
		for i, policyConfig := range policyConfigs {
			policyConfig = strings.TrimSpace(policyConfig)
			if policyConfig == "" {
				continue
			}

			parts := strings.SplitN(policyConfig, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid policy configuration: %s (expected format: policyType:resource1,resource2)", policyConfig)
			}

			policyType := strings.TrimSpace(parts[0])
			resourcesStr := strings.TrimSpace(parts[1])

			template, exists := policyTemplates[policyType]
			if !exists {
				return fmt.Errorf("unknown policy type: %s", policyType)
			}

			// Parse resources
			resources := strings.Split(resourcesStr, ",")
			for j := range resources {
				resources[j] = strings.TrimSpace(resources[j])
			}

			// Generate policy document
			policyDoc := template.Generator(resources)
			policyJSON, err := json.Marshal(policyDoc)
			if err != nil {
				return err
			}

			// Create policy name
			policyName := fmt.Sprintf("%s-%s-%d", roleName, policyType, i)
			policyNames = append(policyNames, policyName)

			// Create IAM policy
			policy, err := iam.NewPolicy(ctx, fmt.Sprintf("policy-%d", i), &iam.PolicyArgs{
				Name:        pulumi.String(policyName),
				Description: pulumi.String(fmt.Sprintf("%s for role %s", template.Description, roleName)),
				Policy:      pulumi.String(string(policyJSON)),
			}, pulumi.Provider(provider))
			if err != nil {
				return err
			}

			policyArns = append(policyArns, policy.Arn)

			// Attach policy to role
			_, err = iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("role-policy-attachment-%d", i), &iam.RolePolicyAttachmentArgs{
				Role:      role.Name,
				PolicyArn: policy.Arn,
			}, pulumi.Provider(provider))
			if err != nil {
				return err
			}
		}

		// Export role information
		ctx.Export("roleArn", role.Arn)
		ctx.Export("roleName", role.Name)
		ctx.Export("roleId", role.UniqueId)
		ctx.Export("accountId", pulumi.String(current.AccountId))

		// Export policy information
		ctx.Export("policyCount", pulumi.Int(len(policyNames)))
		ctx.Export("policyNames", pulumi.ToStringArray(policyNames))

		// Export all policy ARNs as an array
		if len(policyArns) > 0 {
			// Convert []pulumi.StringOutput to []interface{} for pulumi.All
			policyArnInterfaces := make([]interface{}, len(policyArns))
			for i, arn := range policyArns {
				policyArnInterfaces[i] = arn
			}

			ctx.Export("policyArns", pulumi.All(policyArnInterfaces...).ApplyT(func(arns []interface{}) []string {
				result := make([]string, len(arns))
				for i, arn := range arns {
					result[i] = arn.(string)
				}
				return result
			}))
		} else {
			ctx.Export("policyArns", pulumi.ToStringArray([]string{}))
		}

		// Export configuration summary
		ctx.Export("config", pulumi.Map{
			"roleName":    pulumi.String(roleName),
			"description": pulumi.String(description),
			"region":      pulumi.String(region),
		})

		// Export available policy templates for reference
		availableTemplates := make([]string, 0, len(policyTemplates))
		for key := range policyTemplates {
			availableTemplates = append(availableTemplates, key)
		}
		ctx.Export("availablePolicyTemplates", pulumi.ToStringArray(availableTemplates))

		return nil
	})
}
