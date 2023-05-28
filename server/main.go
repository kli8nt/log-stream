package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all connections
	},
}
var Clientset *kubernetes.Clientset
var config *rest.Config
var err error

func main() {
	http.HandleFunc("/logs", handleLogs)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	podName := r.URL.Query().Get("podName")
	if podName == "" {
		http.Error(w, "podName required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("websocket upgrade: %v", err), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")

	flag.Parse()

	if *kubeconfig != "" {
		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			return err
		}

		// create the Clientset
		Clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}
		// Retrieve the CA certificate data

	} else {
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return err
		}
		// creates the Clientset
		Clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}
	}

	req := Clientset.CoreV1().Pods("default").GetLogs(podName, &corev1.PodLogOptions{
		Follow: true,
	})

	logStream, err := req.Stream(r.Context())
	if err != nil {
		log.Fatalf("failed to open log stream: %v", err)
	}
	defer logStream.Close()

	buf := make([]byte, 1024)
	for {
		n, err := logStream.Read(buf)
		if err != nil {
			break
		}

		err = conn.WriteMessage(websocket.TextMessage, buf[:n])
		if err != nil {
			log.Printf("write: %v", err)
			break
		}
		time.Sleep(1 * time.Second)
	}
}
