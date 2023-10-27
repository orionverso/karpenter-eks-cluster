
# AWS EKS Kubernetes Cluster

The main motivation for this cluster is to be able to practice with Kubernetes quickly without having to configure the underlaying infrastructure over and over again.

Some features:

* Karpenter Autoscaling
* Cluster Autoscaling
* Kubecost
* Ebs-controller
* Elb-controller
* Vpc-cni with kubernetes network policies 
* IAM OIDC built-in
* Metric Server for "top" command
* Vpc-endpoints for most used services like AWS ECR







## Deployment

```bash
#These commands are only a guide to running the cluster.
#My recommendation is that you create an account in Pulumi and gain context to know about these commands

curl -fsSL https://get.pulumi.com | sh 

export AWS_ACCESS_KEY_ID=<YOUR_ACCESS_KEY_ID> && \
export AWS_SECRET_ACCESS_KEY=<YOUR_SECRET_ACCESS_KEY> && \ 
export AWS_REGION=<YOUR_AWS_REGION>

#You must first deploy the network where k8s is going to run
cd $(mktemp -d)
git clone https://github.com/orionverso/pulumi-eks-network && cd pulumi-eks-network
pulumi up 

#Setting for this repo
cd $(mktemp -d)
git clone https://github.com/orionverso/pulumi-eks-cluster && cd pulumi-eks-cluster

#Write some configs
pulumi config set ClusterName <YOUR-CLUSTER-NAME>
pulumi config set --secret account <YOUR-ACCOUNT>
pulumi config set org <YOUR-ORGANIZACION> #IMPORTANT FOR CROSS STACK REFERENCES eg. Network STACK
pulumi config set aws:region $AWS_REGION

pulumi up 
```
If you have some problem running the cluster I can help you.
