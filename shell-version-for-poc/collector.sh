#!/usr/bin/env bash

# The CLUSTER_NAME and SLACK_WEBHOOK_URL ENVs are required.
CLUSTER_NAME=${CLUSTER_NAME:-"CLUSTER_NAME undefine"}
SLACK_WEBHOOK_URL=${SLACK_WEBHOOK_URL:-"SLACK_WEBHOOK_URL undefine"} # "https://hooks.slack.com/services/xxxxxx"

SLACK_CHANNEL=${SLACK_CHANNEL:-"restart-info"}
SLACK_USERNAME=${SLACK_USERNAME:-"k8s-pod-restart-info-collector"}
INTERVAL_SECONDS=${INTERVAL_SECONDS:-60} # list pods every INTERVAL_SECONDS
MUTE_LOOP_COUNT=${MUTE_LOOP_COUNT:-10}  # mute slack notification for MUTE_LOOP_COUNT * INTERVAL_SECONDS

loopCount=1

function getPodList() {
    echo "loopCount: $loopCount"
    sleep $INTERVAL_SECONDS
    mv new.txt old.txt >/dev/null 2>&1

    # get all pods which 0<RESTARTS<10
    # Output format: namespace podName restartCount
    kubectl get pod --all-namespaces --no-headers | awk '$5>0 && $5<10{print $1,$2,$5}' | sort >new.txt
    if [ ! -f "old.txt" ]; then
        cp new.txt old.txt
    fi

    # get the new restarted pods
    # Output format: namespace podName
    rm pods_tmp.txt >/dev/null 2>&1
    cat new.txt | while read line; do
        pod=$(echo $line | cut -d " " -f 1,2)
        restartCount=$(echo $line | cut -d " " -f 3)
        if grep -q "$pod" old.txt; then
            OldRestartCount=$(grep "$pod" old.txt | cut -d " " -f 3)
            [ "${restartCount}x" != "${OldRestartCount}x" ] && echo "$pod" >>pods_tmp.txt
        else
            echo "$pod" >>pods_tmp.txt
        fi
    done

    # Remove duplicate pods in histroy file
    if [ -f "pods_tmp.txt" ]; then
        if [ -f "history.txt" -a -s "history.txt" ]; then
            grep -v -f history.txt pods_tmp.txt >pods.txt
        else
            cat pods_tmp.txt >pods.txt
        fi

        if [ -f "pods.txt" -a -s "pods.txt" ]; then
            echo "loopCount:$loopCount" >>history.txt
            cat pods.txt >>history.txt
        fi
    fi

    # Remove old pods in history file, when loopCount > OldLoopCount + MUTE_LOOP_COUNT.
    if [ -f "history.txt" -a -s "history.txt" ]; then
        mv history.txt history_old.txt
        cat history_old.txt | while read line; do
            if echo $line | grep -q "loopCount"; then
                OldLoopCount=$(echo $line | grep "loopCount" | cut -d ":" -f 2)
                let OldLoopCount=OldLoopCount+$MUTE_LOOP_COUNT
            fi

            [ "$loopCount" -lt "$OldLoopCount" ] && echo $line >>history.txt
        done
    fi

    let loopCount=loopCount+1
}

function getAndSendPodInfo() {
    cat pods.txt | while read NAMESPACE POD_NAME; do
        rm status.txt log.txt event.txt >/dev/null 2>&1
        echo "Pod restarted! cluster:$CLUSTER_NAME, podName:$POD_NAME, namespace:$NAMESPACE"

        # get pods current and last status
        echo '```' >>status.txt
        kubectl -n $NAMESPACE get pod $POD_NAME >>status.txt
        echo '```' >>status.txt

        kubectl -n $NAMESPACE get pod $POD_NAME -o json | jq -r '.status.containerStatuses[]|select(.restartCount>0)|"• Reason: \(.lastState.terminated.reason)"' >>status.txt

        echo "• Status" >>status.txt
        echo '```' >>status.txt
        kubectl -n $NAMESPACE get pod $POD_NAME -o json | jq -r '.status.containerStatuses[]|select(.restartCount>0)' | grep -Ev 'containerID|imageID|"image"' | sed 's/{//g;s/},//g;s/}//g;s/"//g;/^[[:space:]]*$/d' | head -n 20 >>status.txt
        echo '```' >>status.txt

        # filter pod events
        echo -e "• Pod Events" >>event.txt
        echo '```' >>event.txt
        kubectl -n $NAMESPACE get event --sort-by=.lastTimestamp --field-selector type!=Normal | grep "$POD_NAME" | sed -E 's/[[:space:]]{2,}/,/g' | tail -n 15 >>event.txt
        echo '```' >>event.txt

        # filter node events
        echo -e "• Node Events" >>event.txt
        echo '```' >>event.txt
        NODE_NAME=$(kubectl -n $NAMESPACE get pod $POD_NAME -o json | jq -r ".spec.nodeName")
        if [ "$NODE_NAME"!="null" -a -n "$NODE_NAME" ]; then
            kubectl get node "$NODE_NAME" >>event.txt
            echo >>event.txt
            kubectl get event --all-namespaces --sort-by=.lastTimestamp | grep "$NODE_NAME" | sed -E 's/[[:space:]]{2,}/,/g' | grep -v "Successfully assigned" | tail -n 15 >>event.txt
        fi
        echo '```' >>event.txt

        # get logs for the previous instance of the containers in a pod
        CONTAINER_NAMES=$(kubectl -n $NAMESPACE  get pod $POD_NAME -o json | jq -r '.status.containerStatuses[]|select(.restartCount>0)|.name')
        echo -e "• Logs Before Restart" >>log.txt
        echo '```' >>log.txt
        for CONTAINER in $CONTAINER_NAMES; do
            kubectl -n $NAMESPACE logs $POD_NAME --container=$CONTAINER --previous --prefix --timestamps --tail 20 >>log.txt
            echo >>log.txt
        done
        echo '```' >>log.txt

        sendToSlackChannel
    done
    rm pods.txt >/dev/null 2>&1
}

function sendToSlackChannel() {
    title="Pod restarted!\ncluster: $CLUSTER_NAME, podname: $POD_NAME, namespace: $NAMESPACE"

    status_text="$(cat status.txt | sed "s/\"/'/g")"
    event_text="$(cat event.txt | sed "s/\"/'/g")"
    log_text="$(cat log.txt | sed "s/\"/'/g")"


    message="$status_text $event_text $log_text"

    attachments=$(
        cat <<-EOF
    [
	    {
            "title": "${title}",
            "type": "mrkdwn",
            "text": "${message}",
            "color": "#4599DF",
            "footer": "$CLUSTER_NAME, $POD_NAME, $NAMESPACE",
            "ts": $(date +%s),
        }
    ]
EOF
    )

    curl -X POST -s --data-urlencode "payload={\"channel\": \"$SLACK_CHANNEL\", \"username\": \"$SLACK_USERNAME\", \"attachments\": $attachments, \"icon_emoji\": \":kubernetes:\"}" "${SLACK_WEBHOOK_URL}"
}

function main() {
    while true; do
        getPodList
        [ -f "pods.txt" -a -s "pods.txt" ] && getAndSendPodInfo
    done
}

main
