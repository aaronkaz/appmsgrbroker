package kube

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func getPods() (interface{}, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	ns, err := SelfNamespace()
	if err != nil {
		return nil, err
	}

	log.Println("current namespace...", ns)

	pods, err := clientset.CoreV1().Pods(ns).List(context.Background(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

	for i, pod := range pods.Items {
		fmt.Printf("[%2d] %s, Phase: %s, Created: %s, HostIP: %s\n", i,
			pod.GetName(), string(pod.Status.Phase),
			pod.GetCreationTimestamp(),
			string(pod.Status.HostIP))
	}

	return nil, nil
}

const cKubernetesNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

func SelfNamespace() (ns string, err error) {
	var fileData []byte
	if fileData, err = os.ReadFile(cKubernetesNamespaceFile); err != nil {
		err = errors.Wrapf(err, "error reading %s; can't get self pod", cKubernetesNamespaceFile)
		return
	}

	ns = strings.TrimSpace(string(fileData))
	return
}
