# k8s-pod-restart-info-collector

k8s-pod-restart-info-collector is a simple K8s customer controller that watches for Pods changes and collects K8s Pod restart reasons, logs, and events to Slack channel when a Pod restarts.

For more information, see the blog on Medium: [Automated Troubleshooting of Kubernetes (K8s) Pods Issues](https://able8.medium.com/automated-troubleshooting-of-kubernetes-pods-issues-c6463bed2f29)

This project is actively used and maintained by Airwallex DevOps team.

## Overview of the Data Collected

Here are two Slack screenshots of the example messages.

### Brief Alert Message
![image](https://miro.medium.com/max/1200/1*iFQeWKHZv3zzJC8lgiZtjA.png)

### Detailed Alert Message

As shown below, by clicking “Show more”, we can see the Reason, “Pod Status”, “Pod Events”, “Node Status and Events”, and “Pod Logs Before Restart”.

![image](https://miro.medium.com/max/1200/1*mvzXhbNeQCJ9Blh1oDH4uw.png)


## How to test and develop locally

```bash
export SLACK_WEBHOOK_URL=https://hooks.slack.com/services/xxxxx/xxxxx
export GOOGLECHAT_WEBHOOK_URL=https://chat.googleapis.com/v1/spaces/xxxxx
go run .
```

## Install using Helm

**Replace the `slackWebhookUrl`, `clusterName` and  `slackChannel`.**

```bash
helm upgrade --install k8s-pod-restart-info-collector ./helm \
   --set slackWebhookUrl="https://hooks.slack.com/services/Change-Me" \
   --set clusterName="Change-Me" \
   --set slackChannel="Change-Me"
```

Check Commands:

```bash
# check commands
kubectl get pod,deploy,sa,secret -l app.kubernetes.io/instance=k8s-pod-restart-info-collector
helm status k8s-pod-restart-info-collector
helm get values k8s-pod-restart-info-collector
helm get manifest k8s-pod-restart-info-collector
helm get all k8s-pod-restart-info-collector
# see logs
kubectl logs deployment/k8s-pod-restart-info-collector -f
```

Run a `debug-pod` to verify the collector:

```bash
kubectl run debug-pod --image=alpine -- date;sleep 30
kubectl get pod debug-pod -w
```

## Uninstall

To uninstall/delete the `k8s-pod-restart-info-collector` helm release:

```bash
helm uninstall k8s-pod-restart-info-collector
```

> The command removes all the Kubernetes components associated with the chart and deletes the release.

## Helm Parameters

| Name                                | Description                                        | Value         |
| ------------------------------------| -------------------------------------------------- | ------------- |
| `clusterName`                       | K8s cluster name (Display on slack message)                        | required         |
| `slackUsername`                     | Slack username (Display on slack message) | default: `"k8s-pod-restart-info-collector"`          |
| `slackChannel`                      | Slack channel name | default: `"restart-info-nonprod"`          |
| `muteSeconds`                       | The time to mute duplicate pod alerts | default: `"600"`    
| `ignoreRestartCount`                | The number of pod restart count to ignore | default: `"30"`
| `ignoredNamespaces`                 | A set of namespaces to be ignored. This should be provided as a comma-separated list or a regular expression. | default: `""`    
| `ignoredPodNamePrefixes`            | A set of pod name prefixes to be ignored. This should be provided as a comma-separated list or a regular expression. | default: `""`   
| `watchedNamespaces`                 | A set of namespaces to be watched. This should be provided as a comma-separated list or a regular expression. | default: `""`    
| `watchedPodNamePrefixes`            | A set of pod name prefixes to be watched. This should be provided as a comma-separated list or a regular expression. | default: `""`   
| `ignoreRestartsWithExitCodeZero`    | Whether restart events with an exit code of 0 should be ignored | default: `false`
| `slackWebhookUrl`                   | Slack webhook URL | required if slackWebhooUrlSecretKeyRef is not present                       |
| `slackWebhookurlSecretKeyRef.key`   | Slack webhook URL SecretKeyRef.key                 | |
| `slackWebhookurlSecretKeyRef.name`  | Slack webhook URL SecretKeyRef.name                | | 
| `useGooglechat`                     | Whether to use Google Chat for notifications | default: `true`  |
| `googlechatWebhookUrl`              | Google Chat webhook URL | required if googlechatWebhookUrlSecretKeyRef is not present  |
| `googlechatWebhookUrlSecretKeyRef.key`   | Google Chat webhook URL SecretKeyRef.key                 | |
| `googlechatWebhookUrlSecretKeyRef.name`  | Google Chat webhook URL SecretKeyRef.name                | |


## FAQ

1. When will the collector send Pod restart messages to Slack channel?

   When a Pod restarts. However, if one of the following conditions is met, the messages are not sent.
   1. Pod restartCount > 30
   2. In the previous 10 minutes, the same Pod restart message was sent

2. How to customize slack channel for each pods

   Adding `alert-slack-channel: "your-slack-channel-name"` to Pod annotations or labels.
   For example, a label: `alert-slack-channel: "restart-info-nonprod"`


## How to write a K8s controller
Please refer to:
- https://github.com/kubernetes/sample-controller/blob/master/docs/controller-client-go.md
- https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/controllers.md
- https://github.com/kubernetes/client-go/tree/master/examples/workqueue

## Copyright and license

Copyright [2022] [Airwallex (Hong Kong) Limited]

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

## Contribution

If you are interested in contributing, see [CONTRIBUTION.md](./CONTRIBUTION.md).
