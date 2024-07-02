
# Iniciamos un cluster de Kubernetes
sudo kubeadm init --pod-network-cidr=10.244.0.0/16

# Configuramos el entorno de Kubernetes
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config

# Instalamos un plugin de red
kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml