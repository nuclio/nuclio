# Install Docker, Helm, kubectl, and Minikube via Homebrew
brew install docker helm kubectl minikube

# Start Minikube
minikube start

# Enable Minikube registry addon - Seems not to function with nuclio properly
# minikube addons enable registry

# Set docker env to minikube, so that we can push images to the minikube registry
eval $(minikube docker-env)

# Run a Docker container to expose Minikube's registry
docker run -d -p 5000:5000 --name function-registry registry:latest

# Build nuclio inside minikube
make build

# Deploy nuclio to the cluster
helm install nuclio \
    --set registry.pushPullUrl=localhost:5000 \
	--set controller.image.tag=latest-arm64 \
	--set dashboard.image.tag=latest-arm64 \
	--set dashboard.baseImagePullPolicy=Never \
	./hack/k8s/helm/nuclio/

# Forward the Nuclio dashboard port
kubectl -n default port-forward $(kubectl get pods -n default -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070


