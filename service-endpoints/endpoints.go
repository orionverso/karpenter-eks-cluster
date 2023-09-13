package network

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type VpcEndpoints struct {
	pulumi.ResourceState
}

type VpcEndpointsArgs struct {
	GatewayEndpointServices   pulumi.StringArrayInput
	InterfaceEndpointServices pulumi.StringArrayInput
	SubnetIds                 pulumi.StringArrayInput
	SecurityGroupIds          pulumi.StringArrayInput
	RouteTableIds             pulumi.StringArrayInput
	VpcId                     pulumi.StringInput
}

func NewVpcEndpoints(ctx *pulumi.Context, name string, args *VpcEndpointsArgs, opts ...pulumi.ResourceOption) (*VpcEndpoints, error) {
	componentResource := &VpcEndpoints{}

	if args == nil {
		args = &VpcEndpointsArgs{}
	}

	// <package>:<module>:<type>
	err := ctx.RegisterComponentResource("my-own-cluster:network:VpcEndpoints", name, componentResource, opts...)
	if err != nil {
		return nil, err
	}
	var interfaceEndpointServices []pulumi.StringInput = []pulumi.StringInput(args.InterfaceEndpointServices.(pulumi.StringArray))

	for _, service := range interfaceEndpointServices {

		_, err = ec2.NewVpcEndpoint(ctx, fmt.Sprintf("vpc-endpoint-%s", service), &ec2.VpcEndpointArgs{
			VpcId:             args.VpcId,
			AutoAccept:        pulumi.BoolPtr(true),
			VpcEndpointType:   pulumi.StringPtr("Interface"),
			ServiceName:       pulumi.Sprintf("com.amazonaws.us-east-1.%s", service),
			SubnetIds:         args.SubnetIds,
			PrivateDnsEnabled: pulumi.BoolPtr(true),
			SecurityGroupIds:  args.SecurityGroupIds,
		}, pulumi.Parent(componentResource))

		if err != nil {
			return nil, err
		}
	}

	var gatewayEndpointServices []pulumi.StringInput = []pulumi.StringInput(args.GatewayEndpointServices.(pulumi.StringArray))

	for _, service := range gatewayEndpointServices {

		_, err = ec2.NewVpcEndpoint(ctx, fmt.Sprintf("vpc-endpoint-%s", service), &ec2.VpcEndpointArgs{
			VpcId:           args.VpcId,
			AutoAccept:      pulumi.BoolPtr(true),
			VpcEndpointType: pulumi.StringPtr("Gateway"),
			ServiceName:     pulumi.Sprintf("com.amazonaws.us-east-1.%s", service),
			RouteTableIds:   args.RouteTableIds,
		}, pulumi.Parent(componentResource))

		if err != nil {
			return nil, err
		}
	}

	ctx.RegisterResourceOutputs(componentResource, pulumi.Map{})

	return componentResource, nil
}
