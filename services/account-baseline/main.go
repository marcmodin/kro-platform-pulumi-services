package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

	"github.com/marcmodin/kro-platform-pulumi-services/pkg/iamrole"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Load configuration
		cfg := config.New(ctx, "")

		// Required: which account can assume these roles
		trustedAccountId := cfg.Require("trustedAccountId")

		// Optional: assume role for deploying into the target account
		assumeRoleArn := cfg.Get("assumeRoleArn")

		// Get AWS region from config
		awsCfg := config.New(ctx, "aws")
		region := awsCfg.Get("region")
		if region == "" {
			region = "eu-north-1"
		}

		// Create AWS provider
		providerArgs := &aws.ProviderArgs{
			Region: pulumi.String(region),
			DefaultTags: &aws.ProviderDefaultTagsArgs{
				Tags: pulumi.StringMap{
					"created_with": pulumi.String("pulumi"),
					"service":      pulumi.String("account-baseline-service"),
					"managed_by":   pulumi.String("pulumi-kubernetes-operator"),
				},
			},
		}

		// If assumeRoleArn is provided, configure cross-account access to deploy
		if assumeRoleArn != "" {
			providerArgs.AssumeRoles = aws.ProviderAssumeRoleArray{
				&aws.ProviderAssumeRoleArgs{
					RoleArn:     pulumi.StringPtr(assumeRoleArn),
					SessionName: pulumi.String("PulumiAccountBaselineService"),
				},
			}
		}

		provider, err := aws.NewProvider(ctx, "aws-provider", providerArgs)
		if err != nil {
			return err
		}

		// Get current AWS account ID (the target account where baseline is deployed)
		current, err := aws.GetCallerIdentity(ctx, &aws.GetCallerIdentityArgs{}, nil, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// Trust policy for both roles
		trustPolicy := iamrole.TrustPolicyArgs{
			Type:            "cross-account",
			TrustedAccounts: []string{trustedAccountId},
		}

		// Create the admin role
		adminRole, err := iamrole.NewRole(ctx, "platform-admin-role", &iamrole.RoleArgs{
			Name:           pulumi.String("PlatformAdminRole"),
			Description:    pulumi.String("Platform cross-account admin role managed by Pulumi"),
			TrustPolicy:    trustPolicy,
			PolicyTemplate: "admin",
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// Create the readonly role
		readonlyRole, err := iamrole.NewRole(ctx, "platform-readonly-role", &iamrole.RoleArgs{
			Name:           pulumi.String("PlatformReadOnlyRole"),
			Description:    pulumi.String("Platform cross-account read-only role managed by Pulumi"),
			TrustPolicy:    trustPolicy,
			PolicyTemplate: "readonly",
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// Export outputs
		ctx.Export("accountId", pulumi.String(current.AccountId))
		ctx.Export("adminRoleArn", adminRole.Arn)
		ctx.Export("adminRoleName", adminRole.Name)
		ctx.Export("readonlyRoleArn", readonlyRole.Arn)
		ctx.Export("readonlyRoleName", readonlyRole.Name)

		return nil
	})
}
