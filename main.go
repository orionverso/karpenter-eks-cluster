package main

import (

	// "k8s-cluster/role"
	"k8s-cluster-own/cluster"
	"k8s-cluster-own/nodegroup"

	// endpoints "k8s-cluster-own/service-endpoints"

	// endpoints "k8s-cluster-own/service-endpoints"

	"fmt"

	"k8s-cluster-own/addon"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/eks"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		cfg := config.New(ctx, "")
		org := cfg.Require("org")
		stack := cfg.Require("stack")

		networkRef, err := pulumi.NewStackReference(ctx, fmt.Sprintf("%s/k8s-network/%s", org, stack), &pulumi.StackReferenceArgs{})
		if err != nil {
			return err
		}

		allsubnets := networkRef.GetOutput(pulumi.String("Subnets")).AsStringArrayOutput()
		privateSubnets := networkRef.GetOutput(pulumi.String("PrivateSubnetIds")).AsStringArrayOutput()
		// publicSubnets:= networkRef.GetOutput(pulumi.String("PublicSubnetIds")).AsStringArrayOutput()
		// privateRouteTableIds := networkRef.GetOutput(pulumi.String("PrivateRouteTableIds")).AsStringArrayOutput()
		vpcId := networkRef.GetOutput(pulumi.String("VpcId")).AsStringOutput()

		principalCluster, err := cluster.NewPrincipalCluster(ctx, "principal-cluster", &cluster.PrincipalClusterArgs{
			SubnetsIds: allsubnets,
			VpcId:      vpcId,
		})

		if err != nil {
			return err
		}

		_, err = nodegroup.NewOpenNodeGroup(ctx, "t2-micro-amd64", &nodegroup.OpenNodeGroupArgs{
			NodeGroupArgs: eks.NodeGroupArgs{
				// AmiType:      pulumi.StringPtr("AL2_ARM_64"),
				ClusterName:  principalCluster.Cluster.Name,
				CapacityType: pulumi.StringPtr("ON_DEMAND"),
				DiskSize:     pulumi.IntPtr(5),
				ScalingConfig: eks.NodeGroupScalingConfigArgs{
					MinSize:     pulumi.Int(2),
					DesiredSize: pulumi.Int(2),
					MaxSize:     pulumi.Int(6),
				},
				Labels: pulumi.ToStringMap(map[string]string{
					"arch": "arm64",
				}),
				InstanceTypes: pulumi.ToStringArray([]string{"t2.micro"}),
				SubnetIds:     privateSubnets,
				Tags: pulumi.StringMap(map[string]pulumi.StringInput{
					"karpenter.sh/discovery": principalCluster.Cluster.Name,
				}),
			},
		})
		if err != nil {
			return err
		}

		// _, err = nodegroup.NewOpenNodeGroup(ctx, "t4g-small-arm64", &nodegroup.OpenNodeGroupArgs{
		// 	NodeGroupArgs: eks.NodeGroupArgs{
		// 		AmiType:      pulumi.StringPtr("AL2_ARM_64"),
		// 		ClusterName:  principalCluster.Cluster.Name,
		// 		CapacityType: pulumi.StringPtr("ON_DEMAND"),
		// 		DiskSize:     pulumi.IntPtr(5),
		// 		ScalingConfig: eks.NodeGroupScalingConfigArgs{
		// 			MinSize:     pulumi.Int(2),
		// 			DesiredSize: pulumi.Int(3),
		// 			MaxSize:     pulumi.Int(6),
		// 		},
		// 		Labels: pulumi.ToStringMap(map[string]string{
		// 			"arch": "arm64",
		// 		}),
		// 		InstanceTypes: pulumi.ToStringArray([]string{"t4g.small"}),
		// 		SubnetIds:     privateSubnets,
		// 	},
		// })
		// if err != nil {
		// 	return err
		// }
		//
		amdGroup, err := nodegroup.NewOpenNodeGroup(ctx, "t2-medium-amd64", &nodegroup.OpenNodeGroupArgs{
			NodeGroupArgs: eks.NodeGroupArgs{
				ClusterName:  principalCluster.Cluster.Name,
				CapacityType: pulumi.StringPtr("SPOT"),
				DiskSize:     pulumi.IntPtr(5),
				// NodeRoleArn:  injected,
				ScalingConfig: eks.NodeGroupScalingConfigArgs{
					MinSize:     pulumi.Int(2),
					DesiredSize: pulumi.Int(2),
					MaxSize:     pulumi.Int(6),
				},
				Labels: pulumi.ToStringMap(map[string]string{
					"arch": "amd64",
				}),
				InstanceTypes: pulumi.ToStringArray([]string{"t2.medium"}),
				SubnetIds:     privateSubnets,
				Tags: pulumi.StringMap(map[string]pulumi.StringInput{
					"karpenter.sh/discovery": principalCluster.Cluster.Name,
				}),
			},
		})
		if err != nil {
			return err
		}

		_, err = addon.NewEbsController(ctx, "ebs-controller", &addon.EbsControllerArgs{
			ClusterName:            principalCluster.Cluster.Name,
			IssuerUrlWithoutPrefix: principalCluster.IssuerUrlWithoutPrefix,
		}, pulumi.DependsOn([]pulumi.Resource{amdGroup}))

		if err != nil {
			return err
		}

		_, err = addon.NewElbController(ctx, "elb-controller", &addon.ElbControllerArgs{
			IssuerUrlWithoutPrefix: principalCluster.IssuerUrlWithoutPrefix,
			ClusterName:            principalCluster.Cluster.Name,
		}, pulumi.DependsOn([]pulumi.Resource{amdGroup}))

		if err != nil {
			return err
		}

		// _, err = eks.NewAddon(ctx, "kubecost", &eks.AddonArgs{
		// 	AddonName:   pulumi.String("kubecost_kubecost"),
		// 	ClusterName: principalCluster.Cluster.Name,
		// }, pulumi.DependsOn([]pulumi.Resource{amdGroup}))
		//
		// if err != nil {
		// 	return err
		// }

		_, err = addon.NewKarpenterAutoScaling(ctx, "kapenter-autoscaling", &addon.KarpenterAutoScalingArgs{
			ClusterName:            principalCluster.Cluster.Name,
			ClusterId:              principalCluster.Cluster.ID(),
			IssuerUrlWithoutPrefix: principalCluster.IssuerUrlWithoutPrefix,
			Subnets:                privateSubnets,
		})

		if err != nil {
			return err
		}

		_, err = addon.NewClusterAutoscaling(ctx, "cluster-autoscaling", &addon.ClusterAutoscalingArgs{
			IssuerUrlWithoutPrefix: principalCluster.IssuerUrlWithoutPrefix,
		})

		// var InterfaceEndpointServices []string = []string{"ecr.api", "ecr.dkr", "sts", "ssm", "ec2messages", "ssmmessages", "ec2"}
		// var GatewayEndpointServices []string = []string{"s3"}
		//
		// _, err = endpoints.NewVpcEndpoints(ctx, "useful-vpc-endpoint-services", &endpoints.VpcEndpointsArgs{
		// 	InterfaceEndpointServices: pulumi.ToStringArray(InterfaceEndpointServices),
		// 	GatewayEndpointServices:   pulumi.ToStringArray(GatewayEndpointServices),
		// 	VpcId:                     vpcId,
		// 	SubnetIds:                 privateSubnets,
		// 	RouteTableIds:             privateRouteTableIds,
		// 	SecurityGroupIds: pulumi.StringArray{principalCluster.Cluster.VpcConfig.ClusterSecurityGroupId().ApplyT(
		// 		func(sgId *string) string {
		// 			return *sgId
		// 		}).(pulumi.StringOutput)},
		// }, pulumi.DependsOn([]pulumi.Resource{principalCluster}))
		// if err != nil {
		// 	return err
		// }

		return nil
	})
}
