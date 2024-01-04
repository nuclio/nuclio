# Install Docker, Helm, kubectl, and Minikube via Homebrew
brew install docker helm kubectl minikube

# Start Minikube
minikube start

# Enable Minikube registry addon
minikube addons enable registry

# Create Nuclio Space
minikube kubectl -- create namespace nuclio

#Add nuclio to helm repo charts
helm repo add nuclio https://nuclio.github.io/nuclio/charts

#Deploy nuclio to the cluster
helm --namespace nuclio install nuclio nuclio/nuclio

# Run a Docker container to expose Minikube's registry
docker run -d --rm --network=host alpine ash -c "apk add socat && socat TCP-LISTEN:5000,reuseaddr,fork TCP:$(minikube ip):5000"

# Forward the Nuclio dashboard port
kubectl -n nuclio port-forward $(kubectl get pods -n nuclio -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070