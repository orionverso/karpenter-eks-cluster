package cluster

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type PrincipalCluster struct {
	pulumi.ResourceState
	IssuerUrlWithoutPrefix pulumi.StringOutput
	oidcProvider           *iam.OpenIdConnectProvider
	Cluster                *eks.Cluster
}

type PrincipalClusterArgs struct {
	SubnetsIds pulumi.StringArrayInput
	VpcId      pulumi.StringInput
}

func NewPrincipalCluster(ctx *pulumi.Context, name string, args *PrincipalClusterArgs, opts ...pulumi.ResourceOption) (*PrincipalCluster, error) {
	componentResource := &PrincipalCluster{}

	if args == nil {
		args = &PrincipalClusterArgs{}
	}

	cfg := config.New(ctx, "aws")
	region := cfg.Require("region")

	// <package>:<module>:<type>
	err := ctx.RegisterComponentResource("my-own-cluster:cluster:PrincipalCluster", name, componentResource, opts...)
	if err != nil {
		return nil, err
	}

	clusterrole, err := iam.NewRole(ctx, fmt.Sprintf("%s-eks-cluster-role", name), &iam.RoleArgs{
		ManagedPolicyArns: pulumi.ToStringArray([]string{"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"}),
		AssumeRolePolicy: pulumi.String(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "eks.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}`),
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	k8scluster, err := eks.NewCluster(ctx, fmt.Sprintf("%s-eks-cluster", name), &eks.ClusterArgs{
		Name:                    pulumi.StringPtr(fmt.Sprintf("%s", name)),
		Version:                 pulumi.StringPtr("1.27"), //TODO: update to 1.28
		KubernetesNetworkConfig: eks.ClusterKubernetesNetworkConfigArgs{},
		VpcConfig: eks.ClusterVpcConfigArgs{
			// VpcId:     k8svpc.ID(),
			SubnetIds: args.SubnetsIds, //implicit vpc
		},
		RoleArn: clusterrole.Arn,
		Tags: pulumi.ToStringMap(map[string]string{
			"karpenter.sh/discovery": fmt.Sprintf("%s", name),
		}),
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	//It is a best practice tag subnets that the cluster is using
	args.SubnetsIds.ToStringArrayOutput().ApplyT(func(subnets []string) error {
		aux := 0
		for _, subnetId := range subnets {
			_, err = ec2.NewTag(ctx, fmt.Sprintf("%s-tag-subnets-with-cluster-%v", name, aux), &ec2.TagArgs{
				Key:        pulumi.Sprintf("kubernetes.io/cluster/%s", k8scluster.Name),
				ResourceId: pulumi.String(subnetId),
				Value:      pulumi.String("owned"),
			}, pulumi.Parent(k8scluster))

			if err != nil {
				return err
			}
			_, err = ec2.NewTag(ctx, fmt.Sprintf("%s-tag-subnets-karpenter-discovery-%v", name, aux), &ec2.TagArgs{
				Key:        pulumi.String("karpenter.sh/discovery"),
				ResourceId: pulumi.String(subnetId),
				Value:      k8scluster.Name,
			}, pulumi.Parent(k8scluster))

			if err != nil {
				return err
			}

			aux = aux + 1
		}
		return nil
	})

	//Karpenter need to know which segurity groups can use by a tag
	_, err = ec2.NewTag(ctx, fmt.Sprintf("%s-TagClusterSecurityGroup", name), &ec2.TagArgs{
		ResourceId: k8scluster.VpcConfig.ClusterSecurityGroupId().Elem().ToStringOutput(),
		Key:        pulumi.String("karpenter.sh/discovery"),
		Value:      k8scluster.Name,
	}, pulumi.Parent(k8scluster), pulumi.DependsOn([]pulumi.Resource{k8scluster}))

	if err != nil {
		return nil, err
	}

	//Create OICD Provider
	IssuerUrl := k8scluster.Identities.Index(pulumi.Int(0)).Oidcs().Index(pulumi.Int(0)).Issuer()

	IssuerUrlWithoutPrefix := IssuerUrl.ApplyT(func(st *string) string {
		if st == nil {
			panic("IssuerUrl is empty")
		}
		stWithoutHttp := strings.Trim(*st, "https://")
		return stWithoutHttp
	}).(pulumi.StringOutput)

	oidcProvider, err := iam.NewOpenIdConnectProvider(ctx, fmt.Sprintf("%s-IdentityProviderOidc", name), &iam.OpenIdConnectProviderArgs{
		ClientIdLists: pulumi.ToStringArray([]string{"sts.amazonaws.com"}),
		Url: IssuerUrl.ApplyT(func(url *string) string {
			return *url
		}).(pulumi.StringOutput),
		ThumbprintLists: pulumi.StringArray{
			k8scluster.Name.ApplyT(func(v string) string {
				if v == "" {
					err := errors.New("Cluster Name is empty")
					fmt.Println(err)
				}
				command := exec.Command("./scripts/thumbprint.sh", v)
				thumbprint, err := command.Output()
				if err != nil {
					fmt.Println(err)
				}
				return string(strings.ToLower(string(thumbprint[:40])))
			}).(pulumi.StringOutput),
		},
	}, pulumi.Parent(k8scluster))

	if err != nil {
		return nil, err
	}

	//Update kubeconfig in local
	kubeconfig := k8scluster.Name.ApplyT(func(name string) string {
		update := exec.Command("./scripts/update-kubeconfig.sh", region, name)
		_, err := update.Output()
		if err != nil {
			fmt.Println(err)
		}
		kubeconfig, err := exec.Command("cat", "~/.kube/config").Output()
		if err != nil {
			fmt.Println(err)
		}
		return string(kubeconfig)

	}).(pulumi.StringOutput)

	if err != nil {
		return nil, err
	}

	componentResource.oidcProvider = oidcProvider
	componentResource.IssuerUrlWithoutPrefix = IssuerUrlWithoutPrefix
	componentResource.Cluster = k8scluster

	ctx.Export("kubeconfig", kubeconfig)
	ctx.Export("IssuerUrl", IssuerUrl)
	ctx.Export("IssuerUrlWithoutPrefix", IssuerUrlWithoutPrefix)
	ctx.Export("ClusterSecurityGroupId", k8scluster.VpcConfig.ClusterSecurityGroupId())
	ctx.Export("ClusterName", k8scluster.Name)

	err = ctx.RegisterResourceOutputs(componentResource, pulumi.Map{
		"ClusterSecurityGroupId": k8scluster.VpcConfig.SecurityGroupIds(),
	})
	if err != nil {
		return nil, err
	}

	return componentResource, nil
}
