## usage for updating kubernetes version

### get source code with full path

```
GO111MODULE=off go get github.com/nuclio/nuclio
GO111MODULE=off go get k8s.io/code-generator 
```

### change versions in go.mod

### gen vendor

```
go mod vendor 
chmod -R 777 vendor 
```

### exec script

```
cd hack ;and sh ./update-codegen.sh 
```


