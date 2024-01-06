# Install Docker, Helm, kubectl, and Minikube via Homebrew
brew install docker helm kubectl minikube

# Start Minikube
minikube start

# Enable Minikube registry addon
minikube addons enable registry

# Add nuclio to helm repo charts
helm repo add nuclio https://nuclio.github.io/nuclio/charts

helm uninstall nuclio --namespace nuclio

#Deploy nuclio to the cluster
helm install nuclio nuclio/nuclio \
    --namespace nuclio \
    --set dashboard.image.repository=quay.io/nuclio/dashboard \
    --set dashboard.image.tag=latest-arm64 \
    --set dashboard.image.pullPolicy=IfNotPresent \
    --set dashboard.image.registry=docker.host.internal:$port_number \
    --set dashboard.image.imageId=56331556f073

# Run a Docker container to expose Minikube's registry
docker run -d -p 5000:5000 --name function-registry registry:latest
#docker run -d --rm --network=host alpine ash -c "apk add socat && socat TCP-LISTEN:5000,reuseaddr,fork TCP:$(minikube ip):5000"

# Forward the Nuclio dashboard port
kubectl -n default port-forward $(kubectl get pods -n default -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070
