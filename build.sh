#!/bin/bash
TAG="v1.2.0"
docker buildx build --platform linux/amd64 --push -t hypatosai/k8s-pod-restart-info-collector:${TAG} .
# docker push hypatosai/k8s-pod-restart-info-collector:${TAG}

# docker tag hypatosai/k8s-pod-restart-info-collector:${TAG} hypatosai/k8s-pod-restart-info-collector:latest
# docker push hypatosai/k8s-pod-restart-info-collector:latest
