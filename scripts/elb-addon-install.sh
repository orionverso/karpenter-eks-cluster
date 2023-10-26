#!/bin/bash

helm repo list | grep "https.*eks-charts" -o | wc -c

if [[  $(helm repo list | grep "https.*eks-charts" -o | wc -c) -eq 0 ]]; then
echo "Adding repository"
helm repo add eks https://aws.github.io/eks-charts
fi
helm repo update eks
helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=$1 \
  --set serviceAccount.create=false \
  --set serviceAccount.name=aws-load-balancer-controller 
