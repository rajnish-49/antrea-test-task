package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	captures = make(map[string]*exec.Cmd)
	mu       sync.Mutex
)

func main() {
	klog.InitFlags(nil)

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		klog.Fatal("NODE_NAME not set")
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, _ := os.UserHomeDir()
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			klog.Fatal(err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	listWatch := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"pods",
		metav1.NamespaceAll,
		fields.OneTermEqualSelector("spec.nodeName", nodeName),
	)

	_, controller := cache.NewInformer(listWatch, &corev1.Pod{}, time.Minute*5, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			checkPod(pod)
		},
		UpdateFunc: func(old, new interface{}) {
			pod := new.(*corev1.Pod)
			checkPod(pod)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			stopCapture(pod.Name)
		},
	})

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		klog.Info("shutting down")
		stopAllCaptures()
		cancel()
	}()

	klog.Infof("watching pods on node %s", nodeName)
	controller.Run(ctx.Done())
}

func checkPod(pod *corev1.Pod) {
	if pod.Status.Phase != corev1.PodRunning {
		return
	}

	annotation, ok := pod.Annotations["tcpdump.antrea.io"]
	if ok {
		startCapture(pod.Name, annotation)
	} else {
		stopCapture(pod.Name)
	}
}

func startCapture(podName string, maxFiles string) {
	mu.Lock()
	defer mu.Unlock()

	if _, running := captures[podName]; running {
		return
	}

	outputFile := fmt.Sprintf("/tmp/capture-%s.pcap", podName)
	cmd := exec.Command("tcpdump", "-C", "1", "-W", maxFiles, "-w", outputFile, "-i", "any")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		klog.Errorf("tcpdump start failed: %v", err)
		return
	}

	captures[podName] = cmd
	klog.Infof("started capture for %s", podName)
}

func stopCapture(podName string) {
	mu.Lock()
	defer mu.Unlock()

	cmd, ok := captures[podName]
	if !ok {
		return
	}

	cmd.Process.Signal(syscall.SIGTERM)
	cmd.Wait()
	delete(captures, podName)

	files, _ := filepath.Glob(fmt.Sprintf("/tmp/capture-%s.pcap*", podName))
	for _, f := range files {
		os.Remove(f)
		klog.Infof("deleted %s", f)
	}
	klog.Infof("stopped capture for %s", podName)
}

func stopAllCaptures() {
	mu.Lock()
	defer mu.Unlock()

	for name, cmd := range captures {
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()
		files, _ := filepath.Glob(fmt.Sprintf("/tmp/capture-%s.pcap*", name))
		for _, f := range files {
			os.Remove(f)
		}
	}
	captures = make(map[string]*exec.Cmd)
}
