sudo dnf update -y # Actualiza el sistema

# Instalacion de Docker
sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo # Agrega el repositorio de Docker
sudo dnf install -y docker-ce docker-ce-cli containerd.io --nobest # Instala Docker

# Iniciar y habilitar Docker
sudo systemctl start docker # Inicia Docker
sudo systemctl enable docker # Habilita Docker para que inicie en el arranque del sistema

# Instalacion de Kubernetes
# Debemos Desactivar el SELinux
sudo setenforce 0
sudo sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config
# Tambien debemos desactivar el SWAP
sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab

# Agregamos la repo de Kubernetes
   curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/arm64/kubectl"

# Instalamos kubectl kubeadm y kubelet
sudo yum install -y kubectl kubeadm kubelet
sudo systemctl enable --now kubelet
sudo systemctl start containerd
sudo systemctl enable containerd

# Abrimos los puertos necesarios
sudo firewall-cmd --add-port=6443/tcp --permanent
sudo firewall-cmd --add-port=10250/tcp --permanent
sudo firewall-cmd --reload