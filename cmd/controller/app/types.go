/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"

	v1beta1 "k8s.io/api/apps/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type functioncrClient interface {
	CreateResource() error
	DeleteResource() error
	WaitForResource() error
	WatchForChanges(namespace string, changeChan chan functioncr.Change) (*functioncr.Watcher, error)
	Create(function *functioncr.Function) (*functioncr.Function, error)
	Update(function *functioncr.Function) (*functioncr.Function, error)
	Delete(namespace, name string, options *meta_v1.DeleteOptions) error
	Get(namespace, name string) (*functioncr.Function, error)
	List(namespace string, options *meta_v1.ListOptions) (*functioncr.FunctionList, error)
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
