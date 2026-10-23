package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestHandlePodReportsRestartedInitContainer(t *testing.T) {
	var requestMu sync.Mutex
	var requestCount int
	var requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read synthetic webhook request: %v", err)
			return
		}
		requestMu.Lock()
		requestBody = string(body)
		requestCount++
		requestMu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "synthetic-pod", Namespace: "synthetic-ns"},
		Spec: v1.PodSpec{
			NodeName: "synthetic-node",
			InitContainers: []v1.Container{{
				Name: "setup",
				Resources: v1.ResourceRequirements{Limits: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("100m"),
				}},
			}},
		},
		Status: v1.PodStatus{
			Phase: v1.PodPending,
			InitContainerStatuses: []v1.ContainerStatus{{
				Name:         "setup",
				RestartCount: 1,
				State:        v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
				LastTerminationState: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{
					Reason: "Error", ExitCode: 1,
				}},
			}},
		},
	}
	clientset := fake.NewSimpleClientset(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "synthetic-node"}})
	controller := &Controller{
		clientset: clientset,
		slack: Slack{
			WebhookUrl:     server.URL,
			DefaultChannel: "synthetic-channel",
			ClusterName:    "synthetic-cluster",
			MuteSeconds:    600,
			History:        make(map[string]time.Time),
		},
	}

	if err := controller.handlePod(pod); err != nil {
		t.Fatalf("handlePod() error = %v", err)
	}
	requestMu.Lock()
	gotCount, body := requestCount, requestBody
	requestMu.Unlock()
	if gotCount != 1 {
		t.Fatalf("Slack webhook requests = %d, want 1 for restarted init container", gotCount)
	}
	for _, want := range []string{"setup", "CrashLoopBackOff", "fake logs", "100m"} {
		if !strings.Contains(body, want) {
			t.Errorf("Slack webhook payload does not contain %q: %s", want, body)
		}
	}
}
