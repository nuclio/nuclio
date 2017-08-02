package app

import (
	"github.com/nuclio/nuclio/pkg/functioncr"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1beta1 "k8s.io/api/apps/v1beta1"
)

type functioncrClient interface {
	CreateResource() error
	DeleteResource() error
	WaitForResource() error
	WatchForChanges(changeChan chan functioncr.Change) (*functioncr.Watcher, error)
	Create(function *functioncr.Function) (*functioncr.Function, error)
	Update(function *functioncr.Function) (*functioncr.Function, error)
	Delete(namespace, name string, options *meta_v1.DeleteOptions) error
	Get(namespace, name string) (*functioncr.Function, error)
	List(namespace string) (*functioncr.FunctionList, error)
}

type functiondepClient interface {
	List(namespace string) ([]v1beta1.Deployment, error)
	Get(namespace string, name string) (*v1beta1.Deployment, error)
	CreateOrUpdate(function *functioncr.Function) (*v1beta1.Deployment, error)
	Delete(namespace string, name string) error
}

type changeIgnorer interface {
	Push(namespacedName string, resourceVersion string)
	Pop(namespacedName string, resourceVersion string) bool
}
