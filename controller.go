package main

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"

	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

const (
	SlackChannelKey = "alert-slack-channel"
)

type Controller struct {
	clientset       kubernetes.Interface
	slack           Slack
	informerFactory informers.SharedInformerFactory
	podInformer     coreinformers.PodInformer
	queue           workqueue.RateLimitingInterface
}

// NewController creates a new Controller.
func NewController(clientset kubernetes.Interface, slack Slack) *Controller {
	const resyncPeriod = 0
	ignoreRestartCount := getIgnoreRestartCount()

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	informerFactory := informers.NewSharedInformerFactory(clientset, resyncPeriod)
	podInformer := informerFactory.Core().V1().Pods()
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old interface{}, new interface{}) {
			oldPod, ok := old.(*v1.Pod)
			if !ok {
				return
			}

			newPod, ok := new.(*v1.Pod)
			if !ok {
				return
			}

			if isIgnoredNamespace(newPod.Namespace) {
				return
			}

			if isIgnoredPod(newPod.Name) {
				return
			}

			newPodRestartCount := getPodRestartCount(newPod)
			// Ignore when restartCount > ignoreRestartCount
			if newPodRestartCount > ignoreRestartCount {
				klog.Infof("Ignore: %s/%s restartCount: %d > %d\n", newPod.Namespace, newPod.Name, newPodRestartCount, ignoreRestartCount)
				return
			}

			oldPodRestartCount := getPodRestartCount(oldPod)
			if newPodRestartCount > oldPodRestartCount {
				key, err := cache.MetaNamespaceKeyFunc(new)
				if err == nil {
					queue.Add(key)
				}
				klog.Infof("Found: %s/%s restarted, restartCount: %d -> %d\n", newPod.Namespace, newPod.Name, oldPodRestartCount, newPodRestartCount)
			}
		},
	})

	return &Controller{
		clientset:       clientset,
		informerFactory: informerFactory,
		podInformer:     podInformer,
		queue:           queue,
		slack:           slack,
	}
}

// Run begins watching and syncing.
func (c *Controller) Run(workers int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()
	klog.Info("Starting controller")

	// Starts all the shared informers that have been created by the factory so
	// far.
	go c.informerFactory.Start(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.podInformer.Informer().HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started controller")

	<-stopCh
	klog.Info("Stopping controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key.
	defer c.queue.Done(key)

	// Invoke the method containing the business logic
	err := c.getAndHandlePod(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)
	return true
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	// This controller retries 3 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 3 {
		klog.Infof("Error syncing Pod %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	klog.Infof("Dropping Pod %q out of the queue: %v", key, err)
}

// getAndHandlePod is the business logic of the controller.
// In case an error happened, it has to simply return the error.
// The retry logic should not be part of the business logic.
func (c *Controller) getAndHandlePod(key string) error {
	pod, err := c.getPodFromIndexer(key)
	if err != nil {
		return err
	}

	err = c.handlePod(pod)
	if err != nil {
		return err
	}
	return nil
}

// getPodFromIndexer retrieves a Pod resource from the indexer with the given key
func (c *Controller) getPodFromIndexer(key string) (*v1.Pod, error) {
	obj, exists, err := c.podInformer.Informer().GetIndexer().GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%s not exists in indexer", key)
	}

	pod, ok := obj.(*v1.Pod)
	if !ok {
		klog.Error("Obj is not a valid Pod object")
		return nil, fmt.Errorf("obj is not a valid Pod object")
	}
	return pod, nil
}

// handlePod collects and sends related info to slack.
func (c *Controller) handlePod(pod *v1.Pod) error {
	// Skip if pod in slack.History
	podKey := pod.Namespace + "/" + pod.Name

	currentTime := time.Now().Local()
	if lastSentTime, ok := c.slack.History[podKey]; ok {
		if int(currentTime.Sub(lastSentTime).Seconds()) < c.slack.MuteSeconds {
			klog.Infof("Skip: %s, already sent %s ago.\n", podKey, duration.HumanDuration(time.Since(lastSentTime)))
			return nil
		}
	}

	// check and collect restarted container info
	for i, status := range pod.Status.ContainerStatuses {
		if status.RestartCount == 0 {
			continue
		}

		klog.Infof("Handle: %s restarted! , restartCount: %d\n\n", podKey, status.RestartCount)

		podInfo, err := printPod(pod)
		if err != nil {
			return err
		}

		containerState, err := describeContainerState(status)
		if err != nil {
			return err
		}

		restartReason := printContainerLastStateReason(status)

		containerSpec := pod.Spec.Containers[i]
		containerResource, err := getContainerResource(containerSpec)
		if err != nil {
			return err
		}

		podStatus := fmt.Sprintf("```%s```\n• Reason: `%s`\n• Pod Status\n```\n%s%s```\n", podInfo, restartReason, containerState, containerResource)
		podEvents, err := c.getPodEvents(pod)
		if err != nil {
			return err
		}
		nodeEvents, err := c.getNodeAndEvents(pod)
		if err != nil {
			return err
		}

		containerLogs, err := c.getContainerLogs(pod, status)
		if err != nil {
			return err
		}
		if containerLogs == "" {
			containerLogs = "• No Logs Before Restart\n"
		} else {
			// Slack attachment text will be truncated when > 8000 chars
			maxLogLength := 7500 - len(podStatus+podEvents+nodeEvents)
			if maxLogLength > 0 && len(containerLogs) > maxLogLength {
				containerLogs = containerLogs[len(containerLogs)-maxLogLength:]
			}
			containerLogs = fmt.Sprintf("• Pod Logs Before Restart\n```\n%s```\n", containerLogs)
		}

		msg := SlackMessage{
			Title:  fmt.Sprintf("*Pod restarted!*\n*cluster: `%s`, pod: `%s`, namespace: `%s`*", c.slack.ClusterName, pod.Name, pod.Namespace),
			Text:   podStatus + podEvents + nodeEvents + containerLogs,
			Footer: fmt.Sprintf("%s, %s, %s", c.slack.ClusterName, pod.Name, pod.Namespace),
		}
		// klog.Infoln(msg.Title + "\n" + msg.Text + "\n" + msg.Footer)
		slackChannel := getSlackChannelFromPod(pod)
		err = c.slack.sendToChannel(msg, slackChannel)
		if err != nil {
			return err
		}

		c.slack.History[podKey] = currentTime
		c.cleanOldSlackHistory()
		break
	}
	return nil
}

func (c *Controller) getPodEvents(pod *v1.Pod) (out string, err error) {
	events, err := c.clientset.CoreV1().Events(pod.Namespace).List(context.TODO(), metav1.ListOptions{FieldSelector: "type!=Normal"})
	if err != nil {
		klog.Error("Failed while getting Pod events.")
		return "", fmt.Errorf("got error while getting events: %v", err)
	}
	// Sort events by their last timestamp
	sortedEvents := events.Items
	if len(sortedEvents) > 1 {
		sort.Sort(byLastTimestamp(sortedEvents))
	}
	for _, event := range sortedEvents {
		if event.InvolvedObject.Name == pod.Name {
			out = out + fmt.Sprintf("%s, %s, %s\n", event.LastTimestamp, event.Reason, event.Message)
		}
	}
	if out == "" {
		out = "• No Warning Pod Events\n"
	} else {
		out = fmt.Sprintf("• Pod Events\n```\n%s```\n", out)
	}
	return out, nil
}

func (c *Controller) getNodeAndEvents(pod *v1.Pod) (out string, err error) {
	node, err := c.clientset.CoreV1().Nodes().Get(context.TODO(), pod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed while getting the %s Node. Probably was deleted. ", pod.Spec.NodeName)
		return "", err
	}
	out, _ = printNode(node)

	events, err := c.clientset.CoreV1().Events(metav1.NamespaceDefault).List(context.TODO(), metav1.ListOptions{FieldSelector: "involvedObject.kind=Node"})
	if err != nil {
		return "", fmt.Errorf("got error while getting events: %v", err)
	}
	// Sort events by their last timestamp
	sortedEvents := events.Items
	if len(sortedEvents) > 1 {
		sort.Sort(byLastTimestamp(sortedEvents))
	}
	for _, event := range sortedEvents {
		if event.InvolvedObject.Name == pod.Spec.NodeName {
			out = out + fmt.Sprintf("%s, %s, %s\n", event.LastTimestamp, event.Reason, event.Message)
		}
	}

	out = fmt.Sprintf("• Node Status and Events\n```\n%s```\n", out)
	return out, nil
}

// getContainerLogs gets previous terminated container logs
func (c *Controller) getContainerLogs(pod *v1.Pod, containerStatus v1.ContainerStatus) (out string, err error) {
	logOptions := &v1.PodLogOptions{
		Container:  containerStatus.Name,
		Previous:   true,
		Timestamps: true,
		TailLines:  pointer.Int64Ptr(50),
	}
	rc, err := c.clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOptions).Stream(context.TODO())
	if err != nil {
		klog.Errorf("got error while getting %s logs: %v", pod.Name, err)
		return "", fmt.Errorf("got error while getting logs: %v", err)
	}
	defer rc.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(rc)

	out = buf.String()
	return out, err
}

// cleanOldSlackHistory deletes old pod name from the c.slack.History.
func (c *Controller) cleanOldSlackHistory() {
	currentTime := time.Now().Local()
	for pod, lastSentTime := range c.slack.History {
		if currentTime.Sub(lastSentTime).Hours() > 1 {
			delete(c.slack.History, pod)
		}
	}
}

// getSlackChannelFromPod gets custom slack channel from pod annotations or labels.
func getSlackChannelFromPod(pod *v1.Pod) string {
	if slackChannel, ok := pod.GetAnnotations()[SlackChannelKey]; ok {
		return slackChannel
	}
	if slackChannel, ok := pod.GetLabels()[SlackChannelKey]; ok {
		return slackChannel
	}
	return ""
}
