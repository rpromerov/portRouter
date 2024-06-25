package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	sessionId := getSessionID(conn)

	fmt.Println("Session ID: ", sessionId)

	ip, port, err := requestRockyLinuxInstance(sessionId)
	if err != nil {
		fmt.Println("Error creating Rocky Linux instance:", err)
	} else {
		fmt.Println("Rocky Linux instance created successfully.")
		fmt.Println("IP: ", ip)
		fmt.Println("Port: ", port)
		rerouteConnection(conn, ip, port, sessionId)
	}

	if err != nil {
		return
	}
}

func rerouteConnection(conn net.Conn, ip string, port int32, sessionId string) {
	fmt.Println("Rerouting connection")
	fmt.Println("IP: ", ip)
	address := fmt.Sprintf("%s:%d", ip, port)
	sshConn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		_, err := io.Copy(sshConn, conn)
		if err != nil {
			fmt.Println("Error: ", err)
		}
		wg.Done()
	}()
	go func() {
		_, err := io.Copy(conn, sshConn)
		if err != nil {
			fmt.Println("Error: ", err)
		}
		wg.Done()
	}()
	time.AfterFunc(10*time.Minute, func() {
		removeDeploymentAndService(sessionId)
	})
	wg.Wait()
	err = sshConn.Close()
	if err != nil {
		fmt.Println("Error: ", err)
	}

}

func requestRockyLinuxInstance(sessionId string) (string, int32, error) {
	serviceName := "rockylinux-ssh-" + sessionId
	config, err := getKubeConfig()
	if err != nil {
		return "", 0, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", 0, err
	}
	ip, port, err := waitForPodAndGetAddress(clientset, sessionId, serviceName, false)
	if ip != "" {
		fmt.Println("Returning existing IP and port")
		return ip, port, nil
	}

	deploymentClient := clientset.AppsV1().Deployments(corev1.NamespaceDefault)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"session": sessionId,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "rockylinux-ssh",
						"session": sessionId,
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
									ContainerPort: 22,
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = deploymentClient.Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		return "", 0, err
	}

	serviceClient := clientset.CoreV1().Services(corev1.NamespaceDefault)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"session": sessionId,
			},
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       22,
					TargetPort: intstr.FromInt(22),
				},
			},
			Type: corev1.ServiceTypeNodePort,
		},
	}
	_, err = serviceClient.Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		return "", 0, err
	}

	// Wait for the pod to be running
	ip, port, err = waitForPodAndGetAddress(clientset, sessionId, serviceName, true)
	if err != nil {
		return "", 0, err
	}
	return ip, port, nil
}
func listenPostgres() {
	fmt.Println("Listening for PostgreSQL connections on port 5432")
	l, err := net.Listen("tcp", "0.0.0.0:5432")
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error: ", err)
			continue
		}
		go handlePostgres(conn)
	}
}

func handlePostgres(conn net.Conn) {
	fmt.Println("Handling connection")
	fmt.Println(conn.LocalAddr())
	fmt.Println(conn.RemoteAddr())
	sessionId := getSessionID(conn)

	fmt.Println("Session ID: ", sessionId)

	ip, port, err := requestPostgresInstance(sessionId)
	if err != nil {
		fmt.Println("Error creating Postgres instance:", err)

	} else {
		fmt.Println("Postgres instance created successfully.")
		fmt.Println("IP: ", ip)
		fmt.Println("Port: ", port)
		rerouteConnection(conn, ip, port, sessionId)
	}

	if err != nil {
		return
	}

}

func requestPostgresInstance(sessionId string) (string, int32, error) {

	serviceName := "postgres-estudiante-" + sessionId
	config, err := getKubeConfig()
	if err != nil {
		return "", 0, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", 0, err
	}
	ip, port, err := waitForPodAndGetAddress(clientset, sessionId, "postgres-estudiante-"+sessionId, false)
	if ip != "" {
		fmt.Println("Returning existing IP and port")
		return ip, port, nil
	}

	deploymentClient := clientset.AppsV1().Deployments(corev1.NamespaceDefault)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"session": sessionId,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "postgres-estudiante",
						"session": sessionId,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{

							Name:            "postgres",
							Image:           "postgres-estudiante",
							ImagePullPolicy: corev1.PullNever,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 5432,
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = deploymentClient.Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		return "", 0, err
	}

	serviceClient := clientset.CoreV1().Services(corev1.NamespaceDefault)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"session": sessionId,
			},
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       5432,
					TargetPort: intstr.FromInt(5432),
				},
			},
			Type: corev1.ServiceTypeNodePort,
		},
	}
	_, err = serviceClient.Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		return "", 0, err
	}

	return waitForPodAndGetAddress(clientset, sessionId, serviceName, true)
}

func waitForPodAndGetAddress(clientset *kubernetes.Clientset, sessionId string, serviceName string, wait bool) (string, int32, error) {

	labelSelector := fmt.Sprintf("session=%s", sessionId)
	for {
		podList, err := clientset.CoreV1().Pods(corev1.NamespaceDefault).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return "", 0, err
		}

		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodRunning {
				node, err := clientset.CoreV1().Nodes().Get(context.TODO(), pod.Spec.NodeName, metav1.GetOptions{})
				if err != nil {
					return "", 0, err
				}
				var nodeIP string
				for _, address := range node.Status.Addresses {
					if address.Type == corev1.NodeInternalIP {
						nodeIP = address.Address
						break
					}
				}
				service, err := clientset.CoreV1().Services(corev1.NamespaceDefault).Get(context.TODO(), serviceName, metav1.GetOptions{})
				if err != nil {
					return "", 0, err
				}
				var nodePort int32
				for _, port := range service.Spec.Ports {
					if port.NodePort != 0 {
						nodePort = port.NodePort
						break
					}
				}
				return nodeIP, nodePort, nil
			}
		}

		fmt.Println("Waiting for pod to be running...")
		if !wait {
			return "", 0, nil
		}
		time.Sleep(1 * time.Second)
	}
}

func removeDeploymentAndService(sessionId string) {
	fmt.Println("Removing deployment and service for session ", sessionId)
	config, err := getKubeConfig()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	deploymentClient := clientset.AppsV1().Deployments(corev1.NamespaceDefault)
	deployments, err := deploymentClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	for _, deployment := range deployments.Items {
		if strings.Contains(deployment.Name, sessionId) {
			err = deploymentClient.Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
			if err != nil {
				fmt.Println("Error: ", err)
				return
			}
		}
	}

	serviceClient := clientset.CoreV1().Services(corev1.NamespaceDefault)
	services, err := serviceClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	for _, service := range services.Items {
		if strings.Contains(service.Name, sessionId) {
			err = serviceClient.Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
			if err != nil {
				fmt.Println("Error: ", err)
				return
			}
		}
	}
}

func getKubeConfig() (*rest.Config, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = clientcmd.RecommendedHomeFile
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}
func getSessionID(conn net.Conn) string {
	remoteIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		fmt.Println("Error: ", err)
		return ""
	}

	_, localPort, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		fmt.Println("Error: ", err)
		return ""
	}

	combined := remoteIP + localPort
	h := md5.New()
	_, err = io.WriteString(h, combined)
	if err != nil {
		fmt.Println("Error: ", err)
		return ""
	}
	hashed := fmt.Sprintf("%x", h.Sum(nil))
	return hashed
}
func int32Ptr(i int32) *int32 { return &i }

func main() {
	fmt.Println("Bienvenid@!")
	go listenSsh()
	listenPostgres()
}
