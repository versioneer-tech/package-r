#!/bin/bash

if [ $# -lt 5 ]; then
  echo "Error: Missing arguments."
  echo "Usage: $0 <name> <aws_access_key_id> <aws_secret_access_key> <aws_endpoint_url> <aws_region>"
  exit 1
fi

NAME="$1"
AWS_ACCESS_KEY_ID="$2"
AWS_SECRET_ACCESS_KEY="$3"
AWS_ENDPOINT_URL="$4"
AWS_REGION="$5"

if [ -z "$NAMESPACE_DEFAULT" ]; then
  if [ ! -f /var/run/secrets/kubernetes.io/serviceaccount/namespace ]; then
    echo "Error: Service account namespace file does not exist."
    exit 1
  fi
  NAMESPACE=$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)
else
  NAMESPACE="$NAMESPACE_DEFAULT"
fi

if kubectl get source "${NAME}" -n "${NAMESPACE}" &> /dev/null; then
  echo "Error: Source '${NAME}' already exists in namespace '${NAMESPACE}'."
  exit 1
fi

if kubectl get secret "${NAME}" -n "${NAMESPACE}" &> /dev/null; then
  echo "Error: Secret '${NAME}' already exists in namespace '${NAMESPACE}'."
  exit 1
fi

if ! kubectl create secret generic "${NAME}" \
  --namespace="${NAMESPACE}" \
  --from-literal=AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" \
  --from-literal=AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" \
  --from-literal=AWS_ENDPOINT_URL="${AWS_ENDPOINT_URL}" \
  --from-literal=AWS_REGION="${AWS_REGION}"; then
  echo "Error: Failed to create secret '${NAME}' in namespace '${NAMESPACE}'."
  exit 1
fi

cat <<EOF | kubectl apply -f -
apiVersion: package.r/alphav1
kind: Source
metadata:
  name: ${NAME}
  namespace: ${NAMESPACE}
spec:
  access:
    secretName: ${NAME}
EOF

if [ $? -ne 0 ]; then
  echo "Error: Failed to apply the Source manifest."
  exit 1
fi

echo "Source object and Secret created successfully!"
