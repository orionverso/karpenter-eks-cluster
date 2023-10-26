#!/bin/bash
aws eks update-kubeconfig --region $1 --name $2

