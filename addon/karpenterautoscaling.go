package addon

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/sqs"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type KarpenterAutoScaling struct {
	pulumi.ResourceState
}

type KarpenterAutoScalingArgs struct {
	ClusterName            pulumi.StringInput
	ClusterId              pulumi.StringInput
	IssuerUrlWithoutPrefix pulumi.StringInput
	Subnets                pulumi.StringArrayInput
}

func NewKarpenterAutoScaling(ctx *pulumi.Context, name string, args *KarpenterAutoScalingArgs, opts ...pulumi.ResourceOption) (*KarpenterAutoScaling, error) {
	componentResource := &KarpenterAutoScaling{}

	if args == nil {
		args = &KarpenterAutoScalingArgs{}
	}

	// <package>:<module>:<type>
	err := ctx.RegisterComponentResource("k8s-cluster:addon:KarpenterAutoScaling", name, componentResource, opts...)
	if err != nil {
		return nil, err
	}

	cfg := config.New(ctx, "")
	account := cfg.GetSecret("account")
	awscfg := config.New(ctx, "aws")
	region := awscfg.Require("region")
	cluster := args.ClusterName

	//curl -fsSL https://raw.githubusercontent.com/aws/karpenter/v0.30.0/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml
	queuename := pulumi.Sprintf("%s", cluster).ToStringPtrOutput()

	InterruptionQueue, err := sqs.NewQueue(ctx, "KarpenterInterruptionQueue", &sqs.QueueArgs{
		Name:                    queuename,
		MessageRetentionSeconds: pulumi.IntPtr(300),
		SqsManagedSseEnabled:    pulumi.BoolPtr(true),
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	_, err = sqs.NewQueuePolicy(ctx, "KarpenterInterruptionQueuePolicy", &sqs.QueuePolicyArgs{
		QueueUrl: InterruptionQueue.Url,
		Policy: pulumi.Sprintf(`{
  "Id": "EC2InterruptionPolicy",
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "Stmt1695685506485",
      "Action": [
        "sqs:SendMessage"
      ],
      "Effect": "Allow",
      "Resource": "%s",
      "Principal": {
        "Service": [
          "events.amazonaws.com",
          "sqs.amazonaws.com"
        ]
      }
    }
  ]
}`, InterruptionQueue.Arn),
	}, pulumi.Parent(InterruptionQueue))

	if err != nil {
		return nil, err
	}

	///eventbridge

	SheduleChangeRule, err := cloudwatch.NewEventRule(ctx, "SheduleChangeRule", &cloudwatch.EventRuleArgs{
		EventPattern: pulumi.StringPtr(`{
  "source": ["aws.health"],
  "detail-type": ["AWS Health Event"]
}`),
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	_, err = cloudwatch.NewEventTarget(ctx, "SheduleChangeRuleTarget", &cloudwatch.EventTargetArgs{
		Arn:  InterruptionQueue.Arn,
		Rule: SheduleChangeRule.Name,
	}, pulumi.Parent(SheduleChangeRule))

	if err != nil {
		return nil, err
	}

	SpotInterruptionRule, err := cloudwatch.NewEventRule(ctx, "SpotInterruptionRule", &cloudwatch.EventRuleArgs{
		EventPattern: pulumi.StringPtr(`{
  "source": ["aws.ec2"],
  "detail-type": ["EC2 Spot Instance Interruption Warning"]
}`),
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	_, err = cloudwatch.NewEventTarget(ctx, "SpotInterruptionRuleTarget", &cloudwatch.EventTargetArgs{
		Arn:  InterruptionQueue.Arn,
		Rule: SpotInterruptionRule.Name,
	}, pulumi.Parent(SpotInterruptionRule))

	if err != nil {
		return nil, err
	}

	RebalanceRule, err := cloudwatch.NewEventRule(ctx, "RebalanceRule", &cloudwatch.EventRuleArgs{
		EventPattern: pulumi.StringPtr(`{
  "source": ["aws.ec2"],
  "detail-type": ["EC2 Instance Rebalance Recommendation"]
}`),
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	_, err = cloudwatch.NewEventTarget(ctx, "RebalanceRuleTarget", &cloudwatch.EventTargetArgs{
		Arn:  InterruptionQueue.Arn,
		Rule: RebalanceRule.Name,
	}, pulumi.Parent(RebalanceRule))

	if err != nil {
		return nil, err
	}

	InstanceStateChangeRule, err := cloudwatch.NewEventRule(ctx, "InstanceStateChangeRule", &cloudwatch.EventRuleArgs{
		EventPattern: pulumi.StringPtr(`{
  "source": ["aws.ec2"],
  "detail-type": ["EC2 Instance State-change Notification"]
}`),
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	_, err = cloudwatch.NewEventTarget(ctx, "InstanceStateChangeRuleTarget", &cloudwatch.EventTargetArgs{
		Arn:  InterruptionQueue.Arn,
		Rule: InstanceStateChangeRule.Name,
	}, pulumi.Parent(InstanceStateChangeRule))

	if err != nil {
		return nil, err
	}

	karpenterNodeRole, err := iam.NewRole(ctx, fmt.Sprintf("%s-generic-groupnode-role", name), &iam.RoleArgs{
		Name: pulumi.Sprintf("KarpenterNodeRole-%s", cluster),
		Path: pulumi.StringPtr("/"),
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

	karpenterNodeInstanceProfile, err := iam.NewInstanceProfile(ctx, "KarpenterNodeInstanceProfile", &iam.InstanceProfileArgs{
		Name: pulumi.Sprintf("KarpenterNodeInstanceProfile-%s", cluster).ToStringPtrOutput(),
		Role: karpenterNodeRole.Name,
		Path: pulumi.StringPtr("/"),
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	// The purpose of this group of nodes is not to launch machines, it is to update aws_auth configMap (iamIdentityMappings for eksctl) with the karpenter node role automatically without intricate scripts. So, after that managed karpenter machines can join to the cluster.
	_, err = eks.NewNodeGroup(ctx, "FalseNodeGroup", &eks.NodeGroupArgs{
		NodeRoleArn:  karpenterNodeRole.Arn,
		ClusterName:  args.ClusterName,
		CapacityType: pulumi.StringPtr("ON_DEMAND"),
		DiskSize:     pulumi.IntPtr(5),
		ScalingConfig: eks.NodeGroupScalingConfigArgs{
			MinSize:     pulumi.Int(0),
			DesiredSize: pulumi.Int(0),
			MaxSize:     pulumi.Int(1),
		},
		InstanceTypes: pulumi.ToStringArray([]string{"t2.micro"}),
		SubnetIds:     args.Subnets,
	})

	if err != nil {
		return nil, err
	}

	KarpenterControllerPolicyJSON := pulumi.Sprintf(`{
  "Version": "2012-10-17",
          "Statement": [
            {
              "Sid": "AllowScopedEC2InstanceActions",
              "Effect": "Allow",
              "Resource": [
                "arn:aws:ec2:%s::image/*",
                "arn:aws:ec2:%s::snapshot/*",
                 "arn:aws:ec2:%s:*:spot-instances-request/*",
                  "arn:aws:ec2:%s:*:security-group/*",
                "arn:aws:ec2:%s:*:subnet/*",
                "arn:aws:ec2:%s:*:launch-template/*"
              ],
              "Action": [
                "ec2:RunInstances",
                "ec2:CreateFleet"
              ]
            },
            {
              "Sid": "AllowScopedEC2LaunchTemplateActions",
              "Effect": "Allow",
              "Resource": "arn:aws:ec2:%s:*:launch-template/*",
              "Action": "ec2:CreateLaunchTemplate",
              "Condition": {
                "StringEquals": {
                  "aws:RequestTag/kubernetes.io/cluster/%s": "owned"
                },
                "StringLike": {
                  "aws:RequestTag/karpenter.sh/provisioner-name": "*"
                }
              }
            },
            {
              "Sid": "AllowScopedEC2InstanceActionsWithTags",
              "Effect": "Allow",
              "Resource": [
                "arn:aws:ec2:%s:*:fleet/*",
                "arn:aws:ec2:%s:*:instance/*",
                "arn:aws:ec2:%s:*:volume/*",
                "arn:aws:ec2:%s:*:network-interface/*"
              ],
              "Action": [
                "ec2:RunInstances",
                "ec2:CreateFleet"
              ],
              "Condition": {
                "StringEquals": {
                  "aws:RequestTag/kubernetes.io/cluster/%s": "owned"
                },
                "StringLike": {
                  "aws:RequestTag/karpenter.sh/provisioner-name": "*"
                }
              }
            },
            {
              "Sid": "AllowScopedResourceCreationTagging",
              "Effect": "Allow",
              "Resource": [
                "arn:aws:ec2:%s:*:fleet/*",
                "arn:aws:ec2:%s:*:instance/*",
                "arn:aws:ec2:%s:*:volume/*",
                "arn:aws:ec2:%s:*:network-interface/*",
                "arn:aws:ec2:%s:*:launch-template/*"
              ],
              "Action": "ec2:CreateTags",
              "Condition": {
                "StringEquals": {
                  "aws:RequestTag/kubernetes.io/cluster/%s": "owned",
                  "ec2:CreateAction": [
                    "RunInstances",
                    "CreateFleet",
                    "CreateLaunchTemplate"
                  ]
                },
                "StringLike": {
                  "aws:RequestTag/karpenter.sh/provisioner-name": "*"
                }
              }
            },
            {
              "Sid": "AllowMachineMigrationTagging",
              "Effect": "Allow",
              "Resource": "arn:aws:ec2:%s:*:instance/*",
              "Action": "ec2:CreateTags",
              "Condition": {
                "StringEquals": {
                  "aws:ResourceTag/kubernetes.io/cluster/%s": "owned",
                  "aws:RequestTag/karpenter.sh/managed-by": "%s"
                },
                "StringLike": {
                  "aws:RequestTag/karpenter.sh/provisioner-name": "*"
                },
                "ForAllValues:StringEquals": {
                  "aws:TagKeys": [
                    "karpenter.sh/provisioner-name",
                    "karpenter.sh/managed-by"
                  ]
                }
              }
            },
            {
              "Sid": "AllowScopedDeletion",
              "Effect": "Allow",
              "Resource": [
                "arn:aws:ec2:%s:*:instance/*",
                "arn:aws:ec2:%s:*:launch-template/*"
              ],
              "Action": [
                "ec2:TerminateInstances",
                "ec2:DeleteLaunchTemplate"
              ],
              "Condition": {
                "StringEquals": {
                  "aws:ResourceTag/kubernetes.io/cluster/%s": "owned"
                },
                "StringLike": {
                  "aws:ResourceTag/karpenter.sh/provisioner-name": "*"
                }
              }
            },
            {
              "Sid": "AllowRegionalReadActions",
              "Effect": "Allow",
              "Resource": "*",
              "Action": [
                "ec2:DescribeAvailabilityZones",
                "ec2:DescribeImages",
                "ec2:DescribeInstances",
                "ec2:DescribeInstanceTypeOfferings",
                "ec2:DescribeInstanceTypes",
                "ec2:DescribeLaunchTemplates",
                "ec2:DescribeSecurityGroups",
                "ec2:DescribeSpotPriceHistory",
                "ec2:DescribeSubnets"
              ],
              "Condition": {
                "StringEquals": {
                  "aws:RequestedRegion": "%s"
                }
              }
            },
            {
              "Sid": "AllowGlobalReadActions",
              "Effect": "Allow",
              "Resource": "*",
              "Action": [
                "pricing:GetProducts",
                "ssm:GetParameter"
              ]
            },
            {
              "Sid": "AllowInterruptionQueueActions",
              "Effect": "Allow",
              "Resource": "%s",
              "Action": [
                "sqs:DeleteMessage",
                "sqs:GetQueueAttributes",
                "sqs:GetQueueUrl",
                "sqs:ReceiveMessage"
              ]
            },
            {
              "Sid": "AllowPassingInstanceRole",
              "Effect": "Allow",
              "Resource": "arn:aws:iam::%s:role/KarpenterNodeRole-%s",
              "Action": "iam:PassRole",
              "Condition": {
                "StringEquals": {
                  "iam:PassedToService": "ec2.amazonaws.com"
                }
              }
            },
            {
              "Sid": "AllowAPIServerEndpointDiscovery",
              "Effect": "Allow",
              "Resource": "arn:aws:eks:%s:%s:cluster/%s",
              "Action": "eks:DescribeCluster"
            }
          ]
  }`, region, region, region, region, region, region, region, cluster, region, region, region, region, cluster, region, region, region, region, region, cluster, region, cluster, cluster, region, region, cluster, region, InterruptionQueue.Arn, account, cluster, region, account, cluster)

	KarpenterControllerPolicy, err := iam.NewPolicy(ctx, "KarpenterConrtrollerPolicy", &iam.PolicyArgs{
		Name:   pulumi.Sprintf("KarpenterControllerPolicy-%s", cluster),
		Policy: KarpenterControllerPolicyJSON,
	}, pulumi.Parent(componentResource))

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
				                    "%s:sub": "system:serviceaccount:karpenter:karpenter"
				                }
				            }
				        }
				    ]
				}`, account, args.IssuerUrlWithoutPrefix, args.IssuerUrlWithoutPrefix, args.IssuerUrlWithoutPrefix)
	//
	KarpenterControllerRole, err := iam.NewRole(ctx, "KarpenterControllerRole", &iam.RoleArgs{
		AssumeRolePolicy:  trustedpolicy,
		ManagedPolicyArns: pulumi.StringArray{KarpenterControllerPolicy.Arn},
	}, pulumi.Parent(componentResource))

	if err != nil {
		return nil, err
	}

	//CREATE BY: helm
	// _, err = corev1.NewNamespace(ctx, "karpenter-ns", &corev1.NamespaceArgs{
	// 	Metadata: metav1.ObjectMetaArgs{
	// 		Name: pulumi.StringPtr("karpenter"),
	// 	},
	// })
	//
	// if err != nil {
	// 	return nil, err
	// }

	// _, err = corev1.NewServiceAccount(ctx, "AllowKarpenterToUseEc2", &corev1.ServiceAccountArgs{
	// 	Metadata: metav1.ObjectMetaArgs{
	// 		Annotations: pulumi.StringMap{
	// 			"eks.amazonaws.com/role-arn":               pulumi.Sprintf("%s", KarpenterControllerRole.Arn),
	// 			"eks.amazonaws.com/sts-regional-endpoints": pulumi.String("true"),
	// 		},
	// 		Name:      pulumi.StringPtr("karpenter"),
	// 		Namespace: pulumi.StringPtr("karpenter"),
	// 	},
	// }, pulumi.Parent(componentResource), pulumi.DependsOn([]pulumi.Resource{karpenterNodeRole, karpenterNamespace}))
	//
	// if err != nil {
	// 	return nil, err
	// }
	//END CREATED BY:

	//automatically implement by eks.NewNodeGroup when create new node groups
	//   iamIdentityMappings:
	// - arn: "arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME}"
	//   username: system:node:{{EC2PrivateDNSName}}
	//   groups:
	//   - system:bootstrappers
	//   - system:nodes

	ctx.Export("KarpenterControllerRoleArn", KarpenterControllerRole.Arn)
	ctx.Export("KarpenterNodeRoleArn", karpenterNodeRole.Arn)
	ctx.Export("KarpenterVersion", pulumi.String("v0.31.0"))
	ctx.Export("KarpenterQueueName", InterruptionQueue.Name)
	ctx.Export("KarpenterNodeInstanceProfileName", karpenterNodeInstanceProfile.Name)

	ctx.RegisterResourceOutputs(componentResource, pulumi.Map{})

	return componentResource, nil
}
