package addon

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type VpcCni struct {
	pulumi.ResourceState
}

type VpcCniArgs struct {
	IssuerUrlWithoutPrefix pulumi.StringInput
	ClusterName            pulumi.StringInput
}

func NewVpcCni(ctx *pulumi.Context, name string, args *VpcCniArgs, opts ...pulumi.ResourceOption) (*VpcCni, error) {
	componentResource := &VpcCni{}

	if args == nil {
		args = &VpcCniArgs{}
	}

	cfg := config.New(ctx, "")
	account := cfg.GetSecret("account")

	// <package>:<module>:<type>
	err := ctx.RegisterComponentResource("my-own-cluster:addon:VpcCni", name, componentResource, opts...)
	if err != nil {
		return nil, err
	}

	trustedpolicy := pulumi.Sprintf(`{
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
				                    "%s:sub": "system:serviceaccount:kube-system:aws-node"
				                }
				            }
				        }
				    ]
				}`, account, args.IssuerUrlWithoutPrefix, args.IssuerUrlWithoutPrefix, args.IssuerUrlWithoutPrefix)
	//
	vpcCniRole, err := iam.NewRole(ctx, "Vpc-cni-role", &iam.RoleArgs{
		AssumeRolePolicy:  trustedpolicy,
		ManagedPolicyArns: pulumi.ToStringArray([]string{"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"}),
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	_, err = corev1.NewServiceAccount(ctx, "VpcCNI-addon-ServiceAccount", &corev1.ServiceAccountArgs{
		Metadata: metav1.ObjectMetaArgs{
			Annotations: pulumi.StringMap{
				"eks.amazonaws.com/role-arn":               pulumi.Sprintf("%s", vpcCniRole.Arn),
				"eks.amazonaws.com/sts-regional-endpoints": pulumi.String("true"),
			},
			Name:      pulumi.StringPtr("aws-node"),
			Namespace: pulumi.StringPtr("kube-system"),
		},
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}
	_, err = eks.NewAddon(ctx, "Vpc-cni-AddOn", &eks.AddonArgs{
		AddonName:                pulumi.String("vpc-cni"),
		AddonVersion:             pulumi.StringPtr("v1.13.4-eksbuild.1"),
		ClusterName:              args.ClusterName,
		ResolveConflictsOnUpdate: pulumi.StringPtr("OVERWRITE"),
		ServiceAccountRoleArn:    vpcCniRole.Arn,
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	ctx.RegisterResourceOutputs(componentResource, pulumi.Map{})

	return componentResource, nil
}
