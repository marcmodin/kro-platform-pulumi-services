package main

import (
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/account"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/budgets"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
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

		// Optional: password policy settings (flat keys for Pulumi K8s operator compatibility)
		passwordPolicyEnabled := cfg.GetBool("passwordPolicyEnabled")
		passwordPolicyMinLength := cfg.GetInt("passwordPolicyMinLength")
		if passwordPolicyMinLength == 0 {
			passwordPolicyMinLength = 14
		}
		passwordPolicyMaxAgeDays := cfg.GetInt("passwordPolicyMaxAgeDays")
		passwordPolicyReusePrevention := cfg.GetInt("passwordPolicyReusePrevention")

		// Optional: enabled regions (comma-separated list of region names to enable)
		enabledRegionsStr := cfg.Get("enabledRegions")

		// Optional: budget settings (flat keys for Pulumi K8s operator compatibility)
		budgetEnabled := cfg.GetBool("budgetEnabled")
		budgetLimitAmount := cfg.Get("budgetLimitAmount")
		if budgetLimitAmount == "" {
			budgetLimitAmount = "100"
		}
		budgetLimitUnit := cfg.Get("budgetLimitUnit")
		if budgetLimitUnit == "" {
			budgetLimitUnit = "USD"
		}
		budgetNotificationEmail := cfg.Get("budgetNotificationEmail")
		budgetThreshold := cfg.GetFloat64("budgetThreshold")
		if budgetThreshold == 0 {
			budgetThreshold = 80
		}

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

		// Trust policy for the readonly role
		trustPolicy := iamrole.TrustPolicyArgs{
			Type:            "cross-account",
			TrustedAccounts: []string{trustedAccountId},
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

		// Export role outputs
		ctx.Export("accountId", pulumi.String(current.AccountId))
		ctx.Export("readonlyRoleArn", readonlyRole.Arn)
		ctx.Export("readonlyRoleName", readonlyRole.Name)

		// Create password policy if enabled
		if passwordPolicyEnabled {
			passwordPolicyArgs := &iam.AccountPasswordPolicyArgs{
				MinimumPasswordLength:        pulumi.Int(passwordPolicyMinLength),
				RequireLowercaseCharacters:   pulumi.Bool(true),
				RequireUppercaseCharacters:   pulumi.Bool(true),
				RequireNumbers:               pulumi.Bool(true),
				RequireSymbols:               pulumi.Bool(true),
				AllowUsersToChangePassword:   pulumi.Bool(true),
				HardExpiry:                   pulumi.Bool(false),
			}

			if passwordPolicyMaxAgeDays > 0 {
				passwordPolicyArgs.MaxPasswordAge = pulumi.Int(passwordPolicyMaxAgeDays)
			}
			if passwordPolicyReusePrevention > 0 {
				passwordPolicyArgs.PasswordReusePrevention = pulumi.Int(passwordPolicyReusePrevention)
			}

			passwordPolicy, err := iam.NewAccountPasswordPolicy(ctx, "account-password-policy", passwordPolicyArgs, pulumi.Provider(provider))
			if err != nil {
				return err
			}

			ctx.Export("passwordPolicyEnabled", pulumi.Bool(true))
			ctx.Export("passwordPolicyMinLength", passwordPolicy.MinimumPasswordLength)
		}

		// Enable/disable regions if specified
		if enabledRegionsStr != "" {
			regions := strings.Split(enabledRegionsStr, ",")
			for _, r := range regions {
				r = strings.TrimSpace(r)
				if r == "" {
					continue
				}

				_, err := account.NewRegion(ctx, "region-"+r, &account.RegionArgs{
					RegionName: pulumi.String(r),
					Enabled:    pulumi.Bool(true),
				}, pulumi.Provider(provider))
				if err != nil {
					return err
				}
			}

			ctx.Export("enabledRegions", pulumi.String(enabledRegionsStr))
		}

		// Create budget if enabled
		if budgetEnabled {
			budgetArgs := &budgets.BudgetArgs{
				BudgetType:  pulumi.String("COST"),
				LimitAmount: pulumi.String(budgetLimitAmount),
				LimitUnit:   pulumi.String(budgetLimitUnit),
				TimeUnit:    pulumi.String("MONTHLY"),
			}

			// Add notification if email is provided
			if budgetNotificationEmail != "" {
				budgetArgs.Notifications = budgets.BudgetNotificationArray{
					&budgets.BudgetNotificationArgs{
						ComparisonOperator: pulumi.String("GREATER_THAN"),
						Threshold:          pulumi.Float64(budgetThreshold),
						ThresholdType:      pulumi.String("PERCENTAGE"),
						NotificationType:   pulumi.String("FORECASTED"),
						SubscriberEmailAddresses: pulumi.StringArray{
							pulumi.String(budgetNotificationEmail),
						},
					},
					&budgets.BudgetNotificationArgs{
						ComparisonOperator: pulumi.String("GREATER_THAN"),
						Threshold:          pulumi.Float64(100),
						ThresholdType:      pulumi.String("PERCENTAGE"),
						NotificationType:   pulumi.String("ACTUAL"),
						SubscriberEmailAddresses: pulumi.StringArray{
							pulumi.String(budgetNotificationEmail),
						},
					},
				}
			}

			budget, err := budgets.NewBudget(ctx, "monthly-cost-budget", budgetArgs, pulumi.Provider(provider))
			if err != nil {
				return err
			}

			ctx.Export("budgetName", budget.Name)
			ctx.Export("budgetLimitAmount", pulumi.String(budgetLimitAmount))
			ctx.Export("budgetLimitUnit", pulumi.String(budgetLimitUnit))
		}

		return nil
	})
}
