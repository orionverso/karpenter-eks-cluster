#!/usr/bin/zsh 
# Logout of helm registry to perform an unauthenticated pull against the public ECR
pulumi stack output ClusterName | read CLUSTER_NAME 
pulumi stack output KarpenterControllerRoleArn | read KARPENTER_IAM_ROLE_ARN 
pulumi stack output KarpenterVersion | read KARPENTER_VERSION 
pulumi stack output KarpenterNodeInstanceProfileName | read INSTANCE_PROFILE 
pulumi stack output KarpenterQueueName | read QUEUE 

echo "CLUSTER_NAME: $CLUSTER_NAME"
echo "KARPENTER_IAM_ROLE_ARN: $KARPENTER_IAM_ROLE_ARN"
echo "KARPENTER_VERSION: $KARPENTER_VERSION"
echo "INSTANCE_PROFILE: $INSTANCE_PROFILE"
echo "QUEUE: $QUEUE"
#
helm registry logout public.ecr.aws

helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version ${KARPENTER_VERSION} --namespace karpenter --create-namespace \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
  --set settings.aws.clusterName=${CLUSTER_NAME} \
  --set settings.aws.defaultInstanceProfile=${INSTANCE_PROFILE} \
  --set settings.aws.interruptionQueueName=${QUEUE} \
  --set controller.resources.requests.cpu=1 \
  --set controller.resources.requests.memory=1Gi \
  --set controller.resources.limits.cpu=1 \
  --set controller.resources.limits.memory=1Gi \
   --set logLevel=debug \
  --wait

