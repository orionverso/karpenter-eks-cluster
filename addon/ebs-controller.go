package addon

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type EbsController struct {
	pulumi.ResourceState
}

type EbsControllerArgs struct {
	ClusterName            pulumi.StringInput
	IssuerUrlWithoutPrefix pulumi.StringInput
}

func NewEbsController(ctx *pulumi.Context, name string, args *EbsControllerArgs, opts ...pulumi.ResourceOption) (*EbsController, error) {
	componentResource := &EbsController{}

	if args == nil {
		args = &EbsControllerArgs{}
	}

	cfg := config.New(ctx, "")
	account := cfg.GetSecret("account")

	// <package>:<module>:<type>
	err := ctx.RegisterComponentResource("k8s-nodes:addon:EbsController", name, componentResource, opts...)
	if err != nil {
		return nil, err
	}

	trustedPolicy := pulumi.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::%s:oidc-provider/%s"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "%s:aud": "sts.amazonaws.com",
          "%s:sub": "system:serviceaccount:kube-system:ebs-csi-controller-sa"
        }
      }
    }
  ]
}`, account, args.IssuerUrlWithoutPrefix, args.IssuerUrlWithoutPrefix, args.IssuerUrlWithoutPrefix)

	ebsControllerRole, err := iam.NewRole(ctx, "ebs-controller-role", &iam.RoleArgs{
		AssumeRolePolicy:  trustedPolicy,
		ManagedPolicyArns: pulumi.ToStringArray([]string{"arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"}),
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	_, err = eks.NewAddon(ctx, "ebs-controller-addon", &eks.AddonArgs{
		ClusterName:           args.ClusterName,
		AddonName:             pulumi.String("aws-ebs-csi-driver"),
		ServiceAccountRoleArn: ebsControllerRole.Arn,
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	ctx.RegisterResourceOutputs(componentResource, pulumi.Map{})

	return componentResource, nil
}
