package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"k8s.io/klog/v2"
)

type Slack struct {
	WebhookUrl     string
	DefaultChannel string // Slack channel name
	Username       string // Slack username (will show in slack message)
	ClusterName    string // Kubernete cluster name (will show in slack message)
	MuteSeconds    int    // The time to mute duplicate alerts
	// History stores sent alerts, key: Namespace/podName, value: sentTime
	History map[string]time.Time
}

type SlackMessage struct {
	Title  string
	Text   string
	Footer string
}

func NewSlack() Slack {
	var slackWebhookUrl, slackChannel, slackUsername, clusterName string

	if slackWebhookUrl = os.Getenv("SLACK_WEBHOOK_URL"); slackWebhookUrl == "" {
		klog.Exit("Environment variable SLACK_WEBHOOK_URL is not set")
	}

	if slackChannel = os.Getenv("SLACK_CHANNEL"); slackChannel == "" {
		slackChannel = "restart-info-nonprod"
		klog.Warningf("Environment variable SLACK_CHANNEL is not set, default: %s\n", slackChannel)
	}

	if slackUsername = os.Getenv("SLACK_USERNAME"); slackUsername == "" {
		slackUsername = "k8s-pod-restart-info-collector"
		klog.Warningf("Environment variable SLACK_USERNAME is not set, default: %s\n", slackUsername)
	}

	if clusterName = os.Getenv("CLUSTER_NAME"); clusterName == "" {
		clusterName = "cluster-name"
		klog.Warningf("Environment variable CLUSTER_NAME is not set, default: %s\n", clusterName)
	}

	muteSeconds, err := strconv.Atoi(os.Getenv("MUTE_SECONDS"))
	if err != nil {
		muteSeconds = 600
		klog.Warningf("Environment variable MUTE_SECONDS is not set, default: %d\n", muteSeconds)
	}

	klog.Infof("Slack Info: channel: %s, username: %s, clustername: %s, muteseconds: %d\n", slackChannel, slackUsername, clusterName, muteSeconds)

	return Slack{
		WebhookUrl:     slackWebhookUrl,
		DefaultChannel: slackChannel,
		Username:       slackUsername,
		ClusterName:    clusterName,
		MuteSeconds:    muteSeconds,
		History:        make(map[string]time.Time),
	}
}

func (s Slack) sendToChannel(msg SlackMessage, slackChannel string) error {
	channel := s.DefaultChannel
	if slackChannel != "" {
		channel = slackChannel
	}

	attachment := slack.Attachment{
		Text:       msg.Text,
		Pretext:    msg.Title,
		Footer:     msg.Footer,
		MarkdownIn: []string{"text", "pretext"},
		Color:      "#4599DF",
		Ts:         json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
	}

	err := slack.PostWebhook(s.WebhookUrl, &slack.WebhookMessage{
		Username:    s.Username,
		Channel:     channel,
		IconEmoji:   ":kubernetes:",
		Attachments: []slack.Attachment{attachment},
	})
	if err != nil {
		klog.Errorf("Sending to Slack channel failed with %v", err)
		return err
	}
	klog.Infof("Sent: [%s] to Slack.\n\n", strings.Replace(msg.Title, "\n", " ", -1))
	return nil
}
