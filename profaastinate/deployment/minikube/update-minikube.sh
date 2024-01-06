make

helm upgrade nuclio \
    --namespace nuclio \
    --set registry.pushPullUrl=localhost:5000 \
    --set controller.image.tag=latest-arm64 \
    --set dashboard.image.tag=latest-arm64 \
    --set dashboard.baseImagePullPolicy=Never \
    ../../hack/k8s/helm/nuclio/

