### Kubernetes/Minikube Notes

#### Requirements

- Docker
- Minikube
- Kubernetes/kubectl

#### Start/stop

> Dieses Dokument ist nicht dafür gedacht, alles auf einmal auszuführen, sondern nur als *quick reference* gedacht.

```shell
# Alles alles löschen
minikube delete --all --purge

# Minikube VM beenden (ohne das Cluster zu löschen)
minikube stop

# Minikube VM starten und Cluster erstellen
minikube start 

# Minikube mit eigenem Registry starten
minikube start --insecure-registry "10.0.0.0/24"
```

`minikube start` konfiguriert kubectl automatisch so, dass es das lokale Cluster verwendet.

#### Deployment erstellen

Über ein Deployment lässt sich der gewünschte Zustand des Clusters beschreiben, welcher dann von Kubernetes automatisch realisiert wird.
Kubernetes erstellt alle für das Deployment relevanten Pods und Container und verwaltet diese.
Diese Beschreibung erfolgt über eine .yaml-Datei.

**Beispiel**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fib-deployment
  labels:
    app: fibfib
spec:
  replicas: 2
  selector:
    matchLabels:
      app: fibfib
  template:
    metadata:
      labels:
        app: fibfib
    spec:
      containers:
      - name: fib-container
        image: fib_server:latest
        imagePullPolicy: Never
        ports:
        - containerPort: 80
```

- **spec**: beschreibt den gewünschten Cluster Zustand
- **replicas**: gibt die Anzahl der Pods an
- **template**: beschreibt die Pods
- **spec/containers**: beschreibt die Container innerhalb der Pods
- **imagePullPolicy**: gibt an, was bei Updates passieren soll. 
Wenn das Docker Image manuell aus dem Host Docker in Minikube geladen wird, soll nicht gepullt werden, da kein Registry in der Config hinterlegt ist.
Wenn der Zugang zu einem Registry hinterlegt ist, kann die *imagePullPolicy* bspw. auf `Always` gesetzt werden.

#### Deployment initialisieren/aktualisieren

```shell
kubectl apply -f <deployment-file-name>.yaml
```

#### Image in Minikube laden

```shell
# Image aus Host Docker laden
minikube image load <ImageName>

# In Minikube verfügbare Images anzeigen
minikube image ls
```

#### Service erstellen, um Applikation außerhalb des Clusters verfügbar zu machen

```shell
# verfügbare Deployments anzeigen
kubectl get deployments

# Service erstellen
kubectl expose deployment/<deployment-name> --type="NodePort" --port 1234

# Informationen anzeigen
kubectl describe services/<service-name>
```

Der beim Erstellen des Services angegebene Port ist der Port innerhalb des Clusters. 
Kubectl zeigt nach Erstellen den von außen aufrufbaren Port an.
Ein Service kann dann beispielsweise mit curl aufgerufen werden.

```shell
curl http://$(minikube ip):<service-port>/<path> -d "hello world"
```

#### Cluster analysieren

```shell
# alle pods anzeigen
kubectl get pods -A

# Container (und mehr) in einem Pod anzeigen
kubectl describe pod <pod-name>

# äquivalent
kubectl describe pods/<pod-name>
```

#### Dashboard für Kubernetes Cluster aufrufen

In einer Shell das ausführen, um auf in Kubernetes laufendes Dashboard zuzugreifen.
```shell
kubectl proxy
```

In einer anderen Shell kann dann das Dashboard gestartet werden.
```shell
minikube dashboard
```

haha I put text inside this document 
oh hi mark

### Sources/Further Reading

- [A definitive guide to Kubernetes image pull policy](https://www.airplane.dev/blog/kubernetes-image-pull-policy)
- [Building Docker images in Kubernetes](https://snyk.io/blog/building-docker-images-kubernetes/)
- [medium-local-docker-image-minikube](https://github.com/mr-pascal/medium-local-docker-image-minikube/tree/master)
- [Two easy ways to use local Docker images in Minikube](https://levelup.gitconnected.com/two-easy-ways-to-use-local-docker-images-in-minikube-cd4dcb1a5379)
- [Registries (Minikube)](https://minikube.sigs.k8s.io/docs/handbook/registry/)
- [Private Docker Registry on Kubernetes: Steps to Set Up](https://www.knowledgehut.com/blog/devops/private-docker-registry#what-is-kubernetes-private-docker-registry?-%C2%A0)
- [Managing Resources (Kubernetes)](https://kubernetes.io/docs/concepts/cluster-administration/manage-deployment/)
- [Using Multi-Node Clusters (Minikube)](https://minikube.sigs.k8s.io/docs/tutorials/multi_node/)
- [Terraform Kubernetes Integration with Minikube](https://medium.com/rahasak/terraform-kubernetes-integration-with-minikube-334c43151931)
- [Working with the Container registry (GitHub Container Registry)](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Distribution Registry (CNCF)](https://distribution.github.io/distribution/)
- [Registry (Docker)](https://docs.docker.com/registry/)
- [Run a Stateless Application Using a Deployment](https://kubernetes.io/docs/tasks/run-application/run-stateless-application-deployment/)
- [Deployments (Kubernetes)](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)
