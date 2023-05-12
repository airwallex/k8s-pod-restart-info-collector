#!/bin/bash
TAG="v1.4.0"
docker buildx build --platform linux/amd64 -t devopsairwallex/k8s-pod-restart-info-collector:${TAG} .
docker push devopsairwallex/k8s-pod-restart-info-collector:${TAG}

docker tag devopsairwallex/k8s-pod-restart-info-collector:${TAG} devopsairwallex/k8s-pod-restart-info-collector:latest
docker push devopsairwallex/k8s-pod-restart-info-collector:latest
