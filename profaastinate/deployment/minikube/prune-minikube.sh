# Stop Minikube (if not already stopped)
minikube stop

# Delete Minikube cluster
minikube delete

# Optionally, delete Minikube's local Docker daemon (this will delete all Docker images)
minikube delete --all --purge

# Optionally, delete the Minikube registry container
docker rm function-registry

# Optionally, remove any leftover files or configurations (use with caution)
rm -rf ~/.minikube
rm -rf ~/.kube
