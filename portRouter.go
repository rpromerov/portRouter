package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"k8s.io/client-go/rest"
	"net"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func listenSsh() {
	fmt.Println("Listening on port 2222")
	l, err := net.Listen("tcp", "0.0.0.0:2222")
	if err != nil {
		fmt.Println("Error: ", err)
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error: ", err)
		}
		go handleSsh(conn)
	}
}

func handleSsh(conn net.Conn) {
	fmt.Println("Handling connection")
	fmt.Println(conn.LocalAddr())
	fmt.Println(conn.RemoteAddr())
	sessionId := getSessionID(conn.RemoteAddr().String())

	fmt.Println("Session ID: ", sessionId)

	err := createRockyLinuxInstance(sessionId)
	if err != nil {
		fmt.Println("Error creating Rocky Linux instance:", err)
	} else {
		fmt.Println("Rocky Linux instance created successfully.")
	}

	err = conn.Close()
	if err != nil {
		return
	}
}

func getSessionID(connString string) string {
	hash := md5.New()
	io.WriteString(hash, connString)
	sessionId := fmt.Sprintf("%x", hash.Sum(nil))
	return sessionId
}

func createRockyLinuxInstance(sessionId string) error {
	config, err := getKubeConfig()
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	deploymentClient := clientset.AppsV1().Deployments(corev1.NamespaceDefault)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rockylinux-ssh-" + sessionId,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "rockylinux-ssh",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "rockylinux-ssh",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "rockylinux",
							Image:           "rockylinux:ssh",
							ImagePullPolicy: corev1.PullNever,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 2224,
								},
							},
						},
					},
				},
			},
		},
	}

	_, err = deploymentClient.Create(context.TODO(), deployment, metav1.CreateOptions{})

	ip, err := getDeploymentIP(clientset, "rockylinux-ssh-"+sessionId)
	fmt.Println("IP: ", ip)
	fmt.Println("Error: ", err)
	return err
}
func getDeploymentIP(clientset *kubernetes.Clientset, deploymentName string) (string, error) {
	_, err := clientset.AppsV1().Deployments(corev1.NamespaceDefault).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	podList, err := clientset.CoreV1().Pods(corev1.NamespaceDefault).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			return pod.Status.PodIP, nil
		}
	}
	return "", nil
}

func getKubeConfig() (*rest.Config, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = clientcmd.RecommendedHomeFile
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

func int32Ptr(i int32) *int32 { return &i }

func main() {
	fmt.Println("Hello, World!")
	listenSsh()

}
