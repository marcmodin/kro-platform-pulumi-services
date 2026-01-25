package iamrole

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// PolicyTemplate represents a predefined policy configuration
type PolicyTemplate struct {
	Name             string
	Description      string
	ManagedPolicyArn string
}

// Available policy templates
var policyTemplates = map[string]PolicyTemplate{
	"admin": {
		Name:             "Administrator Access",
		Description:      "Full administrator access to AWS services and resources",
		ManagedPolicyArn: "arn:aws:iam::aws:policy/AdministratorAccess",
	},
	"readonly": {
		Name:             "View Only Access",
		Description:      "Read-only access to AWS services and resources",
		ManagedPolicyArn: "arn:aws:iam::aws:policy/job-function/ViewOnlyAccess",
	},
}

// RoleArgs defines the inputs for the IAMRole component
type RoleArgs struct {
	// Name of the IAM role
	Name pulumi.StringInput
	// Description of the IAM role
	Description pulumi.StringInput
	// Maximum session duration in seconds (default: 3600)
	MaxSessionDuration pulumi.IntPtrInput
	// Trust policy configuration
	TrustPolicy TrustPolicyArgs
	// Policy template to use (e.g., "admin", "readonly")
	PolicyTemplate string
}

// TrustPolicyArgs defines who can assume the role
type TrustPolicyArgs struct {
	// Type of trust policy: "service", "account", or "cross-account"
	Type string
	// AWS service principals (for Type="service"), e.g., ["ec2.amazonaws.com", "lambda.amazonaws.com"]
	Services []string
	// AWS account IDs that can assume this role (for Type="cross-account")
	TrustedAccounts []string
	// Specific role ARNs that can assume this role (optional, more restrictive)
	TrustedRoleArns []string
	// External ID for additional security (optional)
	ExternalId string
	// Current account ID (for Type="account")
	CurrentAccountId string
}

// Role is a reusable Pulumi component for IAM roles
type Role struct {
	pulumi.ResourceState

	// The IAM role ARN
	Arn pulumi.StringOutput `pulumi:"arn"`
	// The IAM role name
	Name pulumi.StringOutput `pulumi:"name"`
	// The IAM role unique ID
	UniqueId pulumi.StringOutput `pulumi:"uniqueId"`
	// ARNs of attached policies
	PolicyArns pulumi.StringArrayOutput `pulumi:"policyArns"`
}

// NewRole creates a new IAMRole component
func NewRole(ctx *pulumi.Context, name string, args *RoleArgs, opts ...pulumi.ResourceOption) (*Role, error) {
	component := &Role{}
	err := ctx.RegisterComponentResource("pkg:iamrole:Role", name, component, opts...)
	if err != nil {
		return nil, err
	}

	// Validate and get policy template
	template, exists := policyTemplates[args.PolicyTemplate]
	if !exists {
		return nil, fmt.Errorf("unknown policy template: %s (available: %v)", args.PolicyTemplate, AvailablePolicyTemplates())
	}

	// Build assume role policy
	assumeRolePolicy, err := buildAssumeRolePolicy(args.TrustPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to build trust policy: %w", err)
	}

	// Set default max session duration
	maxSessionDuration := args.MaxSessionDuration
	if maxSessionDuration == nil {
		maxSessionDuration = pulumi.IntPtr(3600)
	}

	// Create IAM role
	role, err := iam.NewRole(ctx, name+"-role", &iam.RoleArgs{
		Name:               args.Name,
		Description:        args.Description,
		AssumeRolePolicy:   pulumi.String(assumeRolePolicy),
		MaxSessionDuration: maxSessionDuration,
	}, pulumi.Parent(component))
	if err != nil {
		return nil, err
	}

	// Attach the managed policy from template
	attachment, err := iam.NewRolePolicyAttachment(ctx, name+"-policy", &iam.RolePolicyAttachmentArgs{
		Role:      role.Name,
		PolicyArn: pulumi.String(template.ManagedPolicyArn),
	}, pulumi.Parent(component))
	if err != nil {
		return nil, err
	}

	// Set outputs
	component.Arn = role.Arn
	component.Name = role.Name
	component.UniqueId = role.UniqueId
	component.PolicyArns = pulumi.StringArray{attachment.PolicyArn}.ToStringArrayOutput()

	// Register outputs
	ctx.RegisterResourceOutputs(component, pulumi.Map{
		"arn":        role.Arn,
		"name":       role.Name,
		"uniqueId":   role.UniqueId,
		"policyArns": component.PolicyArns,
	})

	return component, nil
}

// buildAssumeRolePolicy creates the trust policy JSON
func buildAssumeRolePolicy(args TrustPolicyArgs) (string, error) {
	var statement map[string]interface{}

	switch args.Type {
	case "service":
		if len(args.Services) == 0 {
			return "", fmt.Errorf("services required for trust policy type 'service'")
		}
		var servicePrincipal interface{}
		if len(args.Services) == 1 {
			servicePrincipal = args.Services[0]
		} else {
			servicePrincipal = args.Services
		}
		statement = map[string]interface{}{
			"Effect": "Allow",
			"Principal": map[string]interface{}{
				"Service": servicePrincipal,
			},
			"Action": "sts:AssumeRole",
		}

	case "account":
		if args.CurrentAccountId == "" {
			return "", fmt.Errorf("currentAccountId required for trust policy type 'account'")
		}
		statement = map[string]interface{}{
			"Effect": "Allow",
			"Principal": map[string]interface{}{
				"AWS": fmt.Sprintf("arn:aws:iam::%s:root", args.CurrentAccountId),
			},
			"Action": "sts:AssumeRole",
		}

	case "cross-account":
		var principal interface{}

		if len(args.TrustedRoleArns) > 0 {
			// Trust specific role ARNs
			if len(args.TrustedRoleArns) == 1 {
				principal = args.TrustedRoleArns[0]
			} else {
				principal = args.TrustedRoleArns
			}
		} else if len(args.TrustedAccounts) > 0 {
			// Trust account roots
			principals := make([]string, len(args.TrustedAccounts))
			for i, account := range args.TrustedAccounts {
				account = strings.TrimSpace(account)
				principals[i] = fmt.Sprintf("arn:aws:iam::%s:root", account)
			}
			if len(principals) == 1 {
				principal = principals[0]
			} else {
				principal = principals
			}
		} else {
			return "", fmt.Errorf("trustedAccounts or trustedRoleArns required for trust policy type 'cross-account'")
		}

		statement = map[string]interface{}{
			"Effect": "Allow",
			"Principal": map[string]interface{}{
				"AWS": principal,
			},
			"Action": "sts:AssumeRole",
		}

		// Add external ID condition if specified
		if args.ExternalId != "" {
			statement["Condition"] = map[string]interface{}{
				"StringEquals": map[string]interface{}{
					"sts:ExternalId": args.ExternalId,
				},
			}
		}

	default:
		return "", fmt.Errorf("invalid trust policy type: %s (must be 'service', 'account', or 'cross-account')", args.Type)
	}

	policy := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			statement,
		},
	}

	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}

	return string(policyJSON), nil
}

// AvailablePolicyTemplates returns the list of available policy template names
func AvailablePolicyTemplates() []string {
	templates := make([]string, 0, len(policyTemplates))
	for key := range policyTemplates {
		templates = append(templates, key)
	}
	return templates
}

// GetPolicyTemplate returns a policy template by name
func GetPolicyTemplate(name string) (PolicyTemplate, bool) {
	template, exists := policyTemplates[name]
	return template, exists
}
