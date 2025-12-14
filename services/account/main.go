package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	awspulumi "github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	orgspulumi "github.com/pulumi/pulumi-aws/sdk/v7/go/aws/organizations"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiconfig "github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// createMoveToSuspendedOUHook creates a hook function that moves an account to a suspended OU
func createMoveToSuspendedOUHook(region, assumeRoleArn, suspendedOuId string) func(*pulumi.ResourceHookArgs) error {
	return func(args *pulumi.ResourceHookArgs) error {
		fmt.Printf("\nExecuting move to suspended OU hook for account ID: %s\n", string(args.ID))
		
		accountId := string(args.ID)
		if accountId == "" {
			return fmt.Errorf("account ID not found in resource hook args")
		}

		// Create AWS SDK client
		ctx := context.Background()
		awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			return fmt.Errorf("failed to load AWS config: %w", err)
		}

		// If assumeRoleArn is configured, assume the role
		if assumeRoleArn != "" {
			stsClient := sts.NewFromConfig(awsCfg)
			creds := stscreds.NewAssumeRoleProvider(stsClient, assumeRoleArn)
			awsCfg.Credentials = creds
		}

		orgsClient := organizations.NewFromConfig(awsCfg)

		// Get current parent of the account
		listParentsOutput, err := orgsClient.ListParents(ctx, &organizations.ListParentsInput{
			ChildId: &accountId,
		})
		if err != nil {
			return fmt.Errorf("failed to list parents for account %s: %w", accountId, err)
		}

		if len(listParentsOutput.Parents) == 0 {
			return fmt.Errorf("no parent found for account %s", accountId)
		}

		currentParentId := *listParentsOutput.Parents[0].Id

		// Skip if already in the suspended OU
		if currentParentId == suspendedOuId {
			return nil
		}

		// Move account to suspended OU
		_, err = orgsClient.MoveAccount(ctx, &organizations.MoveAccountInput{
			AccountId:           &accountId,
			SourceParentId:      &currentParentId,
			DestinationParentId: &suspendedOuId,
		})
		if err != nil {
			return fmt.Errorf("failed to move account %s to suspended OU %s: %w", accountId, suspendedOuId, err)
		}

		return nil
	}
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Load configuration
		cfg := pulumiconfig.New(ctx, "")
		accountName := cfg.Require("accountName")
		email := cfg.Require("email")
		roleName := cfg.Get("roleName")
		if roleName == "" {
			roleName = "OrganizationAccountAccessRole"
		}
		parentId := cfg.Get("parentId")
		suspendedOuId := cfg.Get("suspendedOuId")
		assumeRoleArn := cfg.Get("assumeRoleArn")

		// Get AWS region from config
		awsCfg := pulumiconfig.New(ctx, "aws")
		region := awsCfg.Get("region")
		if region == "" {
			region = "eu-north-1"
		}

		// Create AWS provider for Organizations API (management account)
		providerArgs := &awspulumi.ProviderArgs{
			Region: pulumi.String(region),
			DefaultTags: &awspulumi.ProviderDefaultTagsArgs{
				Tags: pulumi.StringMap{
					"created_with": pulumi.String("pulumi"),
					"service":      pulumi.String("account-service"),
					"managed_by":   pulumi.String("pulumi-kubernetes-operator"),
				},
			},
		}

		// If assumeRoleArn is provided, configure cross-account access
		if assumeRoleArn != "" {
			providerArgs.AssumeRoles = awspulumi.ProviderAssumeRoleArray{
				&awspulumi.ProviderAssumeRoleArgs{
					RoleArn:     pulumi.StringPtr(assumeRoleArn),
					SessionName: pulumi.String("PulumiAccountService"),
				},
			}
		}

		provider, err := awspulumi.NewProvider(ctx, "aws-provider", providerArgs)
		if err != nil {
			return err
		}

		// Create AWS account in Organizations
		// closeOnDeletion is false - we move to suspended OU via hook instead
		accountArgs := &orgspulumi.AccountArgs{
			Name:            pulumi.String(accountName),
			Email:           pulumi.String(email),
			RoleName:        pulumi.String(roleName),
			CloseOnDeletion: pulumi.Bool(false),
		}

		if parentId != "" {
			accountArgs.ParentId = pulumi.String(parentId)
		}

		// Resource options
		resourceOpts := []pulumi.ResourceOption{
			pulumi.Provider(provider),
			pulumi.RetainOnDelete(true), // Keep account in AWS, don't try to remove from org
		}

		// Register delete hook to move account to suspended OU if configured
		if suspendedOuId != "" {
			hook, err := ctx.RegisterResourceHook(
				"move-to-suspended-ou",
				createMoveToSuspendedOUHook(region, assumeRoleArn, suspendedOuId),
				nil,
			)
			if err != nil {
				return err
			}

			resourceOpts = append(resourceOpts, pulumi.ResourceHooks(&pulumi.ResourceHookBinding{
				BeforeDelete: []*pulumi.ResourceHook{hook},
			}))
		}

		account, err := orgspulumi.NewAccount(ctx, "account", accountArgs, resourceOpts...)
		if err != nil {
			return err
		}

		// Export account information
		ctx.Export("accountId", account.ID())
		ctx.Export("accountArn", account.Arn)
		ctx.Export("accountName", account.Name)
		ctx.Export("accountEmail", account.Email)
		ctx.Export("accountState", account.State)
		ctx.Export("roleName", pulumi.String(roleName))

		// Export configuration summary
		ctx.Export("config", pulumi.Map{
			"accountName":   pulumi.String(accountName),
			"email":         pulumi.String(email),
			"roleName":      pulumi.String(roleName),
			"parentId":      pulumi.String(parentId),
			"suspendedOuId": pulumi.String(suspendedOuId),
			"region":        pulumi.String(region),
		})

		return nil
	})
}
