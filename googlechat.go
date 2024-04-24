package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"k8s.io/klog/v2"
)

type GoogleChat struct {
	WebhookUrl  string
	ClusterName string
	MuteSeconds int
	History     map[string]time.Time
}

type GoogleChatMessage struct {
	Text string `json:"text"`
}

func NewGoogleChat() GoogleChat {
	var webhookUrl, clusterName string

	if webhookUrl = os.Getenv("GOOGLECHAT_WEBHOOK_URL"); webhookUrl == "" {
		klog.Exit("Environment variable GOOGLECHAT_WEBHOOK_URL is not set")
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
	klog.Infof("Clustername: %s, muteseconds: %d\n", clusterName, muteSeconds)

	return GoogleChat{
		WebhookUrl:  webhookUrl,
		ClusterName: clusterName,
		MuteSeconds: muteSeconds,
		History:     make(map[string]time.Time),
	}
}

func (g GoogleChat) sendToRoom(msg GoogleChatMessage) error {
	// Marshal the message into JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Send the HTTP POST request
	resp, err := http.Post(g.WebhookUrl, "application/json", bytes.NewBuffer(data))
	if err != nil {
		klog.Infof("Error sending HTTP POST request: %v", err)
		return err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		klog.Infof("Unexpected status code: %d", resp.StatusCode)
		return err
	}

	klog.Infof("Message sent successfully to Google Chat room")

	return nil
}

func (g GoogleChat) sendToRoomPodStatus(msg GoogleChatMessage) error {
	// Marshal the message into JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Send the HTTP POST request
	resp, err := http.Post(g.WebhookUrl, "application/json", bytes.NewBuffer(data))
	if err != nil {
		klog.Infof("Error sending HTTP POST request: %v", err)
		return err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected PodStatus status code: %d", resp.StatusCode)
		klog.Infof("Unexpected PodStatus status code: %d", resp.StatusCode)
		return err
	}

	klog.Infof("Message PodStatus sent successfully to Google Chat room")

	return nil
}

func (g GoogleChat) sendToRoomPodEvent(msg GoogleChatMessage) error {
	// Marshal the message into JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Send the HTTP POST request
	resp, err := http.Post(g.WebhookUrl, "application/json", bytes.NewBuffer(data))
	if err != nil {
		klog.Infof("Error sending HTTP POST request: %v", err)
		return err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected PodEvent status code: %d", resp.StatusCode)
		klog.Infof("Unexpected PodEvent status code: %d", resp.StatusCode)
		return err
	}

	klog.Infof("Message PodEvent sent successfully to Google Chat room")

	return nil
}

func (g GoogleChat) sendToRoomNodeEvents(msg GoogleChatMessage) error {
	// Marshal the message into JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Send the HTTP POST request
	resp, err := http.Post(g.WebhookUrl, "application/json", bytes.NewBuffer(data))
	if err != nil {
		klog.Infof("Error sending HTTP POST request: %v", err)
		return err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected PodEvent status code: %d", resp.StatusCode)
		klog.Infof("Unexpected PodEvent status code: %d", resp.StatusCode)
		return err
	}

	klog.Infof("Message PodStatus sent successfully to Google Chat room")

	return nil
}

func (g GoogleChat) sendToRoomContainerLogs(msg GoogleChatMessage) error {
	// Marshal the message into JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Send the HTTP POST request
	resp, err := http.Post(g.WebhookUrl, "application/json", bytes.NewBuffer(data))
	if err != nil {
		klog.Infof("Error sending HTTP POST request: %v", err)
		return err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected ContainerLogs status code: %d", resp.StatusCode)
		klog.Infof("Unexpected ContainerLogs status code: %d", resp.StatusCode)
		return err
	}

	klog.Infof("Message ContainerLogs sent successfully to Google Chat room")

	return nil
}
