# Install Docker, Helm, kubectl, and Minikube via Homebrew
brew install docker helm kubectl minikube

# Start Minikube
minikube start

# Enable Minikube registry addon
minikube addons enable registry

# Create Nuclio Space
minikube kubectl -- create namespace nuclio

# Add nuclio to helm repo charts
helm repo add nuclio https://nuclio.github.io/nuclio/charts

# Deploy nuclio to the cluster
helm install nuclio \
    --namespace nuclio \
    --set dashboard.image.repository=quay.io/nuclio/dashboard \
    --set dashboard.image.tag=latest-arm64 \
    --set dashboard.baseImagePullPolicy=Never \
    ../../hack/k8s/helm/nuclio/


# Run a Docker container to expose Minikube's registry
docker run -d -p 5000:5000 --name function-registry registry:latest

# Forward the Nuclio dashboard port
kubectl -n nuclio port-forward $(kubectl get pods -n nuclio -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070

# Upgrade nuclio with the new configuration
helm upgrade nuclio \
    --namespace nuclio \
    --set registry.pushPullUrl=localhost:5000 \
    --set controller.image.tag=latest-arm64 \
    --set dashboard.image.tag=latest-arm64 \
    --set dashboard.baseImagePullPolicy=Never \
    ../../hack/k8s/helm/nuclio/

