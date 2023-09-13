package nodegroup

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type GenericGroupNode struct {
	pulumi.ResourceState
}

type GenericGroupNodeArgs struct {
	ClusterName pulumi.StringInput
	Subnets     pulumi.StringArrayInput
}

func NewGenericGroupNode(ctx *pulumi.Context, name string, args *GenericGroupNodeArgs, opts ...pulumi.ResourceOption) (*GenericGroupNode, error) {
	componentResource := &GenericGroupNode{}

	if args == nil {
		args = &GenericGroupNodeArgs{}
	}

	// <package>:<module>:<type>
	err := ctx.RegisterComponentResource("my-cluster-own:nodegroup:GenericGroupNode", name, componentResource, opts...)
	if err != nil {
		return nil, err
	}

	workerRole, err := iam.NewRole(ctx, "generic-groupnode-role", &iam.RoleArgs{
		ManagedPolicyArns: pulumi.ToStringArray([]string{
			"arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore",       // Provides ssh access to worker nodes via AWS SSM
			"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly", //Provides read-only access to ECR
			"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",               // Amazon VPC CNI Plugin
			"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",          // Amazon EKS worker nodes to connect to Amazon EKS Clusters
		}),
		AssumeRolePolicy: pulumi.String(`{
		    "Version": "2012-10-17",
		    "Statement": [
		        {
		            "Effect": "Allow",
		            "Principal": {
		                "Service": "ec2.amazonaws.com"
		            },
		            "Action": "sts:AssumeRole"
		        }
		    ]
		}`),
	}, pulumi.Parent(componentResource))
	if err != nil {
		return nil, err
	}

	// ec2.NewLaunchTemplate(ctx, "GenericNodeGroupLauncTemplate", &ec2.LaunchTemplateArgs{
	//interesting explore options!!
	// })

	_, err = eks.NewNodeGroup(ctx, "genericGroupNode", &eks.NodeGroupArgs{
		AmiType:      pulumi.StringPtr("AL2_ARM_64"),
		ClusterName:  args.ClusterName,
		CapacityType: pulumi.StringPtr("ON_DEMAND"),
		DiskSize:     pulumi.IntPtr(5),
		NodeRoleArn:  workerRole.Arn,
		ScalingConfig: eks.NodeGroupScalingConfigArgs{
			MinSize:     pulumi.Int(2),
			DesiredSize: pulumi.Int(4),
			MaxSize:     pulumi.Int(6),
		},
		InstanceTypes: pulumi.ToStringArray([]string{"t4g.small"}),
		SubnetIds:     args.Subnets,
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
