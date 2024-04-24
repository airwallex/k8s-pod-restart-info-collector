package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

func main() {
	err := godotenv.Load(".env") // Load environment variables from .env file
	if err != nil {
		klog.Warningf("Error loading .env file: %v\n", err)
	}
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		// use InClusterConfig
		config, err = rest.InClusterConfig()
		if err != nil {
			klog.Fatal(err)
		}
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}
	// Get the value of USE_GOOGLE_CHAT from environment variables
	useGoogleChatStr := os.Getenv("USE_GOOGLE_CHAT")
	useGoogleChat := true // default value is true
	if useGoogleChatStr != "" {
		useGoogleChat = (useGoogleChatStr == "true")
	}
	var controller *Controller

	if useGoogleChat {
		googleChat := NewGoogleChat()
		controller = NewControllerGooglechat(clientset, googleChat)
	} else {
		slack := NewSlack()
		controller = NewController(clientset, slack)
	}

	// Start the controller
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	// Wait forever
	select {}

}
