package nodegroup

import (
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type OpenNodeGroup struct {
	pulumi.ResourceState
}

type OpenNodeGroupArgs struct {
	NodeGroupArgs eks.NodeGroupArgs
}

func NewOpenNodeGroup(ctx *pulumi.Context, name string, args *OpenNodeGroupArgs, opts ...pulumi.ResourceOption) (*OpenNodeGroup, error) {
	componentResource := &OpenNodeGroup{}

	if args == nil {
		args = &OpenNodeGroupArgs{}
	}

	// <package>:<module>:<type>
	err := ctx.RegisterComponentResource("k8s-cluster:nodegroup:OpenNodeGroup", name, componentResource, opts...)
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
	args.NodeGroupArgs.NodeRoleArn = workerRole.Arn

	_, err = eks.NewNodeGroup(ctx, "genericGroupNode", &args.NodeGroupArgs, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	ctx.RegisterResourceOutputs(componentResource, pulumi.Map{})

	return componentResource, nil
}
