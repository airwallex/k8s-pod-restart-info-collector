package main

import (
	"bytes"
	"fmt"
	"io"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/describe"
)

func getPodRestartCount(pod *v1.Pod) int {
	var restarts int = 0
	for i := range pod.Status.ContainerStatuses {
		container := pod.Status.ContainerStatuses[i]
		restarts += int(container.RestartCount)
	}
	return restarts
}

func isIgnoredNamespace(namespace string) bool {
	ignoredNamespacesEnv := os.Getenv("IGNORED_NAMESPACES")
	if ignoredNamespacesEnv == "" {
		return false
	}
	ignoredNamespaces := strings.Split(ignoredNamespacesEnv, ",")
	for _, ignoredNamespace := range ignoredNamespaces {
		match, _ := regexp.MatchString(ignoredNamespace, namespace)
		if match {
			klog.Infof("Ignore: namespace %s is in the ignored namespace list\n", namespace)
			return true
		}
	}
	return false
}

func isIgnoredPod(name string) bool {
	ignoredPodNamePrefixesEnv := os.Getenv("IGNORED_POD_NAME_PREFIXES")
	if ignoredPodNamePrefixesEnv == "" {
		return false
	}
	ignoredPodNamePrefixes := strings.Split(ignoredPodNamePrefixesEnv, ",")
	for _, ignoredPodNamePrefix := range ignoredPodNamePrefixes {
		match, _ := regexp.MatchString(ignoredPodNamePrefix, name)
		if match {
			klog.Infof("Ignore: pod %s has ignored name prefix: %s\n", name, ignoredPodNamePrefix)
			return true
		}
	}
	return false
}

func isIgnoredErrorForPod(podName string, errorLog string) bool {
	ignoredErrorsForPodNamePrefixesEnv := os.Getenv("IGNORED_ERRORS_FOR_POD_NAME_PREFIXES")
	if ignoredErrorsForPodNamePrefixesEnv == "" {
		return false
	}

	podErrorsMap := make(map[string][]interface{})
	err := json.Unmarshal([]byte(ignoredErrorsForPodNamePrefixesEnv), &podErrorsMap)
	if err != nil {
		klog.Infof("Failed to load IGNORED_ERRORS_FOR_POD_NAME_PREFIXES with error: %s", err)
		return false
	}

	for key, errors := range podErrorsMap {
		if strings.HasPrefix(podName, key) {
			for _, ignoredError := range errors {
				if strings.Contains(errorLog, ignoredError.(string)) {
					klog.Infof("Ignore: pod %s has ignored error: %s\n", podName, ignoredError)
					return true
				}
			}
		}
	}

	return false
}

func lastNonEmptyLogLine(logs string) string {
	logLines := strings.Split(logs, "\n")

	for i := 1; i <= len(logLines); i++ {
		lastLogLine := logLines[len(logLines)-i]
		if lastLogLine != "" {
			return lastLogLine;
		}
	}

	return ""
}

func isWatchedNamespace(namespace string) bool {
	watchedNamespacesEnv := os.Getenv("WATCHED_NAMESPACES")
	if watchedNamespacesEnv == "" {
		return true
	}
	watchedNamespaces := strings.Split(watchedNamespacesEnv, ",")
	for _, watchedNamespace := range watchedNamespaces {
		match, _ := regexp.MatchString(watchedNamespace, namespace)
		if match {
			return true
		}
	}

	// Turn off logging as there are too many logs.
	// klog.Infof("Ignore: namespace %s is not on the watched namespace list\n", namespace)
	return false
}

func isWatchedPod(name string) bool {
	watchedPodNamePrefixesEnv := os.Getenv("WATCHED_POD_NAME_PREFIXES")
	if watchedPodNamePrefixesEnv == "" {
		return true
	}
	watchedPodNamePrefixes := strings.Split(watchedPodNamePrefixesEnv, ",")
	for _, watchedPodNamePrefix := range watchedPodNamePrefixes {
		match, _ := regexp.MatchString(watchedPodNamePrefix, name)
		if match {
			return true
		}
	}
	// klog.Infof("Ignore: pod %s doesn't have the watched pod name prefixes\n", name)
	return false
}

func shouldIgnoreRestartsWithExitCodeZero(status v1.ContainerStatus) bool {
	if os.Getenv("IGNORE_RESTARTS_WITH_EXIT_CODE_ZERO") != "true" {
		return false
	}

	if status.LastTerminationState.Terminated != nil && status.LastTerminationState.Terminated.ExitCode == 0 {
		return true
	}
	return false
}

func getIgnoreRestartCount() int {
	ignoreRestartCount, err := strconv.Atoi(os.Getenv("IGNORE_RESTART_COUNT"))
	if err != nil {
		ignoreRestartCount = 30
		klog.Warningf("Environment variable IGNORE_RESTART_COUNT is not set, default: %d\n", ignoreRestartCount)
	}
	return ignoreRestartCount
}

func printPod(pod *v1.Pod) (string, error) {
	restarts := 0
	totalContainers := len(pod.Spec.Containers)
	readyContainers := 0
	lastRestartDate := metav1.NewTime(time.Time{})

	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	initializing := false
	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]
		restarts += int(container.RestartCount)
		if container.LastTerminationState.Terminated != nil {
			terminatedDate := container.LastTerminationState.Terminated.FinishedAt
			if lastRestartDate.Before(&terminatedDate) {
				lastRestartDate = terminatedDate
			}
		}
		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			// initialization is failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + container.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}
	if !initializing {
		restarts = 0
		hasRunning := false
		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]

			restarts += int(container.RestartCount)
			if container.LastTerminationState.Terminated != nil {
				terminatedDate := container.LastTerminationState.Terminated.FinishedAt
				if lastRestartDate.Before(&terminatedDate) {
					lastRestartDate = terminatedDate
				}
			}
			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				reason = container.State.Waiting.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
				reason = container.State.Terminated.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else if container.Ready && container.State.Running != nil {
				hasRunning = true
				readyContainers++
			}
		}

		// change pod status back to "Running" if there is at least one container still reporting as "Running" status
		if reason == "Completed" && hasRunning {
			if hasPodReadyCondition(pod.Status.Conditions) {
				reason = "Running"
			} else {
				reason = "NotReady"
			}
		}
	}

	if pod.DeletionTimestamp != nil && pod.Status.Reason == "NodeLost" {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}

	restartsStr := strconv.Itoa(restarts)
	if !lastRestartDate.IsZero() {
		restartsStr = fmt.Sprintf("%d (%s ago)", restarts, translateTimestampSince(lastRestartDate))
	}

	return tabbedString(func(out io.Writer) error {
		w := describe.NewPrefixWriter(out)
		w.Write(describe.LEVEL_0, "NAME\tREADY\tSTATUS\tRESTARTS\tAGE\n")
		w.Write(describe.LEVEL_0, "%s\t%d/%d\t%s\t%s\t%s\n", pod.Name, readyContainers, totalContainers, reason, restartsStr, translateTimestampSince(pod.CreationTimestamp))
		return nil
	})
}

func printContainerLastStateReason(status v1.ContainerStatus) string {
	var lastStateReason string
	var lastExitCode int32
	if status.LastTerminationState.Terminated != nil {
		lastStateReason = status.LastTerminationState.Terminated.Reason
		lastExitCode = status.LastTerminationState.Terminated.ExitCode
	}
	return fmt.Sprintf("%s (ExitCode %d)", lastStateReason, lastExitCode)
}

func printNode(obj *v1.Node) (string, error) {
	conditionMap := make(map[v1.NodeConditionType]*v1.NodeCondition)
	NodeAllConditions := []v1.NodeConditionType{v1.NodeReady}
	for i := range obj.Status.Conditions {
		cond := obj.Status.Conditions[i]
		conditionMap[cond.Type] = &cond
	}
	var status []string
	for _, validCondition := range NodeAllConditions {
		if condition, ok := conditionMap[validCondition]; ok {
			if condition.Status == v1.ConditionTrue {
				status = append(status, string(condition.Type))
			} else {
				status = append(status, "Not"+string(condition.Type))
			}
		}
	}
	if len(status) == 0 {
		status = append(status, "Unknown")
	}
	if obj.Spec.Unschedulable {
		status = append(status, "SchedulingDisabled")
	}

	return tabbedString(func(out io.Writer) error {
		w := describe.NewPrefixWriter(out)
		w.Write(describe.LEVEL_0, "NAME\tSTATUS\tAGE\tVERSION\n")
		w.Write(describe.LEVEL_0, "%s\t%s\t%s\t%s\n", obj.Name, strings.Join(status, ","), translateTimestampSince(obj.CreationTimestamp), obj.Status.NodeInfo.KubeletVersion)
		return nil
	})
}

func hasPodReadyCondition(conditions []v1.PodCondition) bool {
	for _, condition := range conditions {
		if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// translateTimestampSince returns the elapsed time since timestamp in
// human-readable approximation.
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}

func describeContainerState(status v1.ContainerStatus) (string, error) {
	return tabbedString(func(out io.Writer) error {
		w := describe.NewPrefixWriter(out)
		w.Write(describe.LEVEL_0, "%v:\n", status.Name)
		w.Write(describe.LEVEL_1, "Ready:\t%v\n", printBool(status.Ready))
		w.Write(describe.LEVEL_1, "Restart Count:\t%d\n", status.RestartCount)
		describeStatus("State", status.State, w)
		if status.LastTerminationState.Terminated != nil {
			describeStatus("Last State", status.LastTerminationState, w)
		}
		return nil
	})
}

func describeStatus(stateName string, state v1.ContainerState, w describe.PrefixWriter) {
	switch {
	case state.Running != nil:
		w.Write(describe.LEVEL_1, "%s:\tRunning\n", stateName)
		w.Write(describe.LEVEL_2, "Started:\t%v\n", state.Running.StartedAt.Time.Format(time.RFC1123Z))
	case state.Waiting != nil:
		w.Write(describe.LEVEL_1, "%s:\tWaiting\n", stateName)
		if state.Waiting.Reason != "" {
			w.Write(describe.LEVEL_2, "Reason:\t%s\n", state.Waiting.Reason)
		}
	case state.Terminated != nil:
		w.Write(describe.LEVEL_1, "%s:\tTerminated\n", stateName)
		if state.Terminated.Reason != "" {
			w.Write(describe.LEVEL_2, "Reason:\t%s\n", state.Terminated.Reason)
		}
		if state.Terminated.Message != "" {
			w.Write(describe.LEVEL_2, "Message:\t%s\n", state.Terminated.Message)
		}
		w.Write(describe.LEVEL_2, "Exit Code:\t%d\n", state.Terminated.ExitCode)
		if state.Terminated.Signal > 0 {
			w.Write(describe.LEVEL_2, "Signal:\t%d\n", state.Terminated.Signal)
		}
		w.Write(describe.LEVEL_2, "Started:\t%s\n", state.Terminated.StartedAt.Time.Format(time.RFC1123Z))
		w.Write(describe.LEVEL_2, "Finished:\t%s\n", state.Terminated.FinishedAt.Time.Format(time.RFC1123Z))
	default:
		w.Write(describe.LEVEL_1, "%s:\tWaiting\n", stateName)
	}
}

func tabbedString(f func(out io.Writer) error) (string, error) {
	out := new(tabwriter.Writer)
	buf := &bytes.Buffer{}
	out.Init(buf, 0, 8, 2, ' ', 0)

	err := f(out)
	if err != nil {
		return "", err
	}

	err = out.Flush()
	if err != nil {
		return "", err
	}
	str := string(buf.String())
	return str, nil
}

func printBool(value bool) string {
	if value {
		return "True"
	}

	return "False"
}

// byFirstTimestamp sorts a slice of events by first timestamp, using their involvedObject's name as a tie breaker.
type byLastTimestamp []v1.Event

func (o byLastTimestamp) Len() int      { return len(o) }
func (o byLastTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

func (o byLastTimestamp) Less(i, j int) bool {
	if o[i].LastTimestamp.Equal(&o[j].LastTimestamp) {
		return o[i].InvolvedObject.Name < o[j].InvolvedObject.Name
	}
	return o[i].LastTimestamp.Before(&o[j].LastTimestamp)
}

func getContainerResource(container v1.Container) (string, error) {
	return tabbedString(func(out io.Writer) error {
		w := describe.NewPrefixWriter(out)
		resources := container.Resources
		if len(resources.Limits) > 0 {
			w.Write(describe.LEVEL_1, "Limits:\n")
		}
		for _, name := range SortedResourceNames(resources.Limits) {
			quantity := resources.Limits[name]
			w.Write(describe.LEVEL_2, "%s:\t%s\n", name, quantity.String())
		}

		if len(resources.Requests) > 0 {
			w.Write(describe.LEVEL_1, "Requests:\n")
		}
		for _, name := range SortedResourceNames(resources.Requests) {
			quantity := resources.Requests[name]
			w.Write(describe.LEVEL_2, "%s:\t%s\n", name, quantity.String())
		}
		return nil
	})
}

// SortedResourceNames returns the sorted resource names of a resource list.
func SortedResourceNames(list v1.ResourceList) []v1.ResourceName {
	resources := make([]v1.ResourceName, 0, len(list))
	for res := range list {
		resources = append(resources, res)
	}
	sort.Sort(SortableResourceNames(resources))
	return resources
}

type SortableResourceNames []v1.ResourceName

func (list SortableResourceNames) Len() int {
	return len(list)
}

func (list SortableResourceNames) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

func (list SortableResourceNames) Less(i, j int) bool {
	return list[i] < list[j]
}
