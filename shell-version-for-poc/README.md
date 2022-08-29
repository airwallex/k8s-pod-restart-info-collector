
# k8s-pod-restart-info-collector [POC]

A shell script and k8s helm chart to collect the logs and relevant debugging info when k8s Pods restart.

`collector.sh` is the only main script to collect and send info to slack channel.

Other are helm chart files.

## Installing the Collector via Helm

**Replace the `slackWebhookUrl`, `clusterName` and  `slackChannel`.**

```bash
helm install k8s-pod-restart-info-collector ./ \
   --set slackWebhookUrl="https://hooks.slack.com/services/Change-Me" \
   --set clusterName="Change-Me" \
   --set slackChannel="Change-Me"

# upgrade command when the collector.sh file or configs change
helm upgrade k8s-pod-restart-info-collector ./
```

Check and Test Commands:

```bash
# check commands
kubectl get pod,deploy,sa,cm,secret -l app.kubernetes.io/instance=k8s-pod-restart-info-collector
helm status k8s-pod-restart-info-collector
helm get values k8s-pod-restart-info-collector
helm get manifest k8s-pod-restart-info-collector
helm get all k8s-pod-restart-info-collector
# see logs
kubectl logs deployment/k8s-pod-restart-info-collector -f

# create a fake restarted pod, will receive a slack message in `intervalSeconds` seconds.
kubectl exec deployment/k8s-pod-restart-info-collector -- bash -c "echo testNamespace testPod > pods.txt"
```

Run a `debug-pod` to test the collector:

```bash
kubectl delete pod debug-pod
kubectl run debug-pod --image=alpine -- date;sleep 30
kubectl get pod debug-pod -w
```


## Uninstalling the Chart

To uninstall/delete the `k8s-pod-restart-info-collector` helm release:

```bash
helm uninstall k8s-pod-restart-info-collector
```

> The command removes all the Kubernetes components associated with the chart and deletes the release.

## Helm Parameters

| Name                       | Description                                        | Value         |
| -------------------------- | -------------------------------------------------- | ------------- |
| `slackWebhookUrl`     | Slack webhook URL | required          |
| `clusterName`         | Kubernete cluster name(will show in slack message)                        | required         |
| `slackUsername`       | Slack username(will show in slack message) | default: `"k8s-pod-restart-info-collector"`          |
| `slackChannel`        | Slack channel name | default: `"restart-info"`          |
| `intervalSeconds`  |  Interval in seconds to list and check restarted pods | default: `"60"`          |
| `muteLoopCount`    | Mute duplicate pod messages for `muteLoopCount*intervalSeconds` seconds | default: `"10"`          |
