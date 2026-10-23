package main

import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestGetPodRestartCountIncludesInitContainers(t *testing.T) {
	pod := &v1.Pod{Status: v1.PodStatus{
		InitContainerStatuses: []v1.ContainerStatus{{Name: "setup", RestartCount: 2}},
		ContainerStatuses:     []v1.ContainerStatus{{Name: "app", RestartCount: 3}},
	}}

	if got, want := getPodRestartCount(pod), 5; got != want {
		t.Fatalf("getPodRestartCount() = %d, want %d", got, want)
	}
}
