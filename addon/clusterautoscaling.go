package addon

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type ClusterAutoscaling struct {
	pulumi.ResourceState
}

type ClusterAutoscalingArgs struct {
	IssuerUrlWithoutPrefix pulumi.StringInput
}

func NewClusterAutoscaling(ctx *pulumi.Context, name string, args *ClusterAutoscalingArgs, opts ...pulumi.ResourceOption) (*ClusterAutoscaling, error) {
	componentResource := &ClusterAutoscaling{}

	if args == nil {
		args = &ClusterAutoscalingArgs{}
	}

	// <package>:<module>:<type>
	err := ctx.RegisterComponentResource("k8s-cluster:addon:ClusterAutoscaling", name, componentResource, opts...)
	if err != nil {
		return nil, err
	}

	// https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/aws/examples/cluster-autoscaler-autodiscover.yaml

	cfg := config.New(ctx, "")
	account := cfg.Require("account")

	serviceAccountName := "cluster-autoscaler"
	namespace := "kube-system"
	issuerurlwithoutprefix := args.IssuerUrlWithoutPrefix

	policy := pulumi.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "autoscaling:DescribeAutoScalingGroups",
        "autoscaling:DescribeAutoScalingInstances",
        "autoscaling:DescribeLaunchConfigurations",
        "autoscaling:DescribeScalingActivities",
        "autoscaling:DescribeTags",
        "ec2:DescribeInstanceTypes",
        "ec2:DescribeLaunchTemplateVersions"
      ],
      "Resource": ["*"]
    },
    {
      "Effect": "Allow",
      "Action": [
        "autoscaling:SetDesiredCapacity",
        "autoscaling:TerminateInstanceInAutoScalingGroup",
        "ec2:DescribeImages",
        "ec2:GetInstanceTypesFromInstanceRequirements",
        "eks:DescribeNodegroup"
      ],
      "Resource": ["*"]
    }
  ]
}`).ToStringPtrOutput()

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
          "%s:sub": "system:serviceaccount:%s:%s"
        }
      }
    }
  ]
}`, account, issuerurlwithoutprefix, issuerurlwithoutprefix, issuerurlwithoutprefix, namespace, serviceAccountName)

	//Resources Example
	role, err := iam.NewRole(ctx, fmt.Sprintf("%s-cluster-autoscaler-ASG", name), &iam.RoleArgs{
		AssumeRolePolicy: trustedPolicy,
		InlinePolicies: iam.RoleInlinePolicyArray{
			iam.RoleInlinePolicyArgs{
				Name:   pulumi.StringPtr("AllowClusterAutoscaling"),
				Policy: policy,
			},
		},
	})

	if err != nil {
		return nil, err
	}

	metaArgs := metav1.ObjectMetaArgs{
		Annotations: pulumi.StringMap{
			"eks.amazonaws.com/role-arn":               pulumi.Sprintf("%s", role.Arn),
			"eks.amazonaws.com/sts-regional-endpoints": pulumi.String("true"),
		},
		Name:      pulumi.StringPtr(serviceAccountName),
		Namespace: pulumi.StringPtr(namespace),
		Labels: pulumi.ToStringMap(map[string]string{
			"k8s-addon": "cluster-autoscaler.addons.k8s.io",
			"k8s-app":   "cluster-autoscaler",
		}),
	}

	_, err = corev1.NewServiceAccount(ctx, fmt.Sprintf("%s-cluster-autoscaler", name), &corev1.ServiceAccountArgs{
		Metadata: metaArgs,
	}, pulumi.DependsOn([]pulumi.Resource{role}), pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	ctx.RegisterResourceOutputs(componentResource, pulumi.Map{})

	return componentResource, nil
}
