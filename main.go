package main

import (

	// "k8s-cluster/role"
	"k8s-cluster-own/cluster"
	"k8s-cluster-own/nodegroup"
	endpoints "k8s-cluster-own/service-endpoints"

	// endpoints "k8s-cluster-own/service-endpoints"

	"fmt"

	"k8s-cluster-own/addon"

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
		privateRouteTableIds := networkRef.GetOutput(pulumi.String("PrivateRouteTableIds")).AsStringArrayOutput()
		vpcId := networkRef.GetOutput(pulumi.String("VpcId")).AsStringOutput()

		principalCluster, err := cluster.NewPrincipalCluster(ctx, "principal-cluster", &cluster.PrincipalClusterArgs{
			SubnetsIds: allsubnets,
		})

		if err != nil {
			return err
		}

		_, err = addon.NewVpcCni(ctx, "vpc-cni", &addon.VpcCniArgs{
			ClusterName:            principalCluster.Cluster.Name,
			IssuerUrlWithoutPrefix: principalCluster.IssuerUrlWithoutPrefix,
		}, pulumi.Parent(principalCluster))

		if err != nil {
			return err
		}

		_, err = addon.NewEbsController(ctx, "ebs-controller", &addon.EbsControllerArgs{
			ClusterName:            principalCluster.Cluster.Name,
			IssuerUrlWithoutPrefix: principalCluster.IssuerUrlWithoutPrefix,
		}, pulumi.Parent(principalCluster))

		if err != nil {
			return err
		}

		_, err = addon.NewElbController(ctx, "elb-controller", &addon.ElbControllerArgs{
			IssuerUrlWithoutPrefix: principalCluster.IssuerUrlWithoutPrefix,
			ClusterName:            principalCluster.Cluster.Name,
		}, pulumi.Parent(principalCluster))

		if err != nil {
			return err
		}

		var InterfaceEndpointServices []string = []string{"ecr.api", "ecr.dkr", "sts", "ssm", "ec2messages", "ssmmessages", "ec2"}
		var GatewayEndpointServices []string = []string{"s3"}

		_, err = endpoints.NewVpcEndpoints(ctx, "useful-vpc-endpoint-services", &endpoints.VpcEndpointsArgs{
			InterfaceEndpointServices: pulumi.ToStringArray(InterfaceEndpointServices),
			GatewayEndpointServices:   pulumi.ToStringArray(GatewayEndpointServices),
			VpcId:                     vpcId,
			SubnetIds:                 privateSubnets,
			RouteTableIds:             privateRouteTableIds,
			SecurityGroupIds: pulumi.StringArray{principalCluster.Cluster.VpcConfig.ClusterSecurityGroupId().ApplyT(
				func(sgId *string) string {
					return *sgId
				}).(pulumi.StringOutput)},
		}, pulumi.DependsOn([]pulumi.Resource{principalCluster}))
		if err != nil {
			return err
		}

		_, err = nodegroup.NewGenericGroupNode(ctx, "genericGroupNode", &nodegroup.GenericGroupNodeArgs{
			ClusterName: principalCluster.Cluster.Name,
			Subnets:     privateSubnets,
		}, pulumi.DependsOn([]pulumi.Resource{principalCluster}))
		if err != nil {
			return err
		}

		return nil
	})
}
