# En el nodo de control debemos obtener el token de acceso para unir los nodos al cluster. Para ello ejecutamos el siguiente comando:
kubeadm token create --print-join-command

# Ahora insertamos el comando que nos dio el nodo de control en el nodo trabajador
sudo kubeadm join <ip-controlador>:<puerto-control> --token <token> --discovery-token-ca-cert-hash sha256:<hash>