#!/bin/bash

#!/bin/bash

set -e

if [ ! -z "$DEBUG" ] ; then
    set -x
fi

CLUSTER_NAME=$1

# Get cluster OIDC Provider for the cluster using AWS CLI
OIDC_PROVIDER=$(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.identity.oidc.issuer" --output text)

# Ask OIDC Provider for JWKS host (remove schema and path with sed)
JWKS_URI=$(curl -s ${OIDC_PROVIDER}/.well-known/openid-configuration | jq -r '.jwks_uri' | sed -e "s/^https:\/\///" | sed 's/\/.*//')

# Extract all certificates in separate files
# https://unix.stackexchange.com/questions/368123/how-to-extract-the-root-ca-and-subordinate-ca-from-a-certificate-chain-in-linux
TEMP=$(mktemp -d -t oidc-eks-XXXX)
openssl s_client -servername $JWKS_URI -showcerts -connect $JWKS_URI:443 < /dev/null 2>/dev/null | awk -v dir="$TEMP" '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/{ if(/BEGIN/){a++}; out=dir"/cert00"a".crt"; print >out }'

# Assume last found certificate in chain is the ROOT_CA
ROOT_CA=$(ls -1 $TEMP/* | tail -1)

# Extract fingerprint in desired format (no header, no colons)
THUMBPRINT=$(openssl x509 -fingerprint -noout -in $ROOT_CA | sed 's/^.*=//' | sed 's/://g')

# printf '{"thumbprint": "%s"}\n' $THUMBPRINT
echo $THUMBPRINT

rm -rf $TEMP

