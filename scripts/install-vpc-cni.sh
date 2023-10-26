#!/bin/bash
#You want to update to helm aws-vpc-cni because the automanaged not implement network policy
#delete automanaged  aws-vpc-cni
kubectl delete serviceaccounts -n kube-system aws-node
kubectl delete clusterroles.rbac.authorization.k8s.io aws-node
kubectl delete clusterrolebindings.rbac.authorization.k8s.io aws-node
kubectl delete daemonset aws-node --namespace kube-system

helm repo add eks https://aws.github.io/eks-charts
helm install aws-vpc-cni --namespace kube-system eks/aws-vpc-cni
helm upgrade --install --force aws-vpc-cni --namespace kube-system eks/aws-vpc-cni
helm upgrade --set enableNetworkPolicy=true aws-vpc-cni --namespace kube-system eks/aws-vpc-cni
