#!/usr/bin/env bash

set -xe

source /root/.bash_profile

WAITSECS=${WAITSECS:-"2"}

for i in {1..20}
do
  if kubectl get services
  then
    exit 0
  fi
  echo "Waiting another ${WAITSECS} seconds for k8s API to be up..."
  sleep $WAITSECS
done

echo "Waiting for the RKE2 ingress controller to be up..."
kubectl rollout status daemonset \
  rke2-ingress-nginx-controller \
  -n kube-system \
  --timeout 600s
