#!/bin/bash
for a in operator istioctl galley mixer_codegen mixer test_policybackend app_sidecar app proxyv2 pilot
do
   docker image rm -f $(docker images | grep $a | awk '{print$3}')
done
