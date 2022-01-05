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

package platformconfig

import (
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/v3io/scaler/pkg/scalertypes"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	machinarymetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const DefaultFunctionReadinessTimeoutSeconds = 60

type LoggerSink struct {
	Kind       string                 `json:"kind,omitempty"`
	URL        string                 `json:"url,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type LoggerSinkWithLevel struct {
	Level string
	Sink  LoggerSink
}

type LoggerSinkBinding struct {
	Level string `json:"level,omitempty"`
	Sink  string `json:"sink,omitempty"`
}

type FunctionsLogger struct {
	DefaultLevel string `json:"defaultLevel,omitempty"`
	DefaultSink  string `json:"defaultSink,omitempty"`
}

type Logger struct {
	Sinks     map[string]LoggerSink `json:"sinks,omitempty"`
	System    []LoggerSinkBinding   `json:"system,omitempty"`
	Functions []LoggerSinkBinding   `json:"functions,omitempty"`
}

type WebServer struct {
	Enabled       *bool  `json:"enabled,omitempty"`
	ListenAddress string `json:"listenAddress,omitempty"`
}

type MetricSink struct {
	Enabled    *bool                  `json:"enabled,omitempty"`
	Kind       string                 `json:"kind,omitempty"`
	URL        string                 `json:"url,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type ScaleToZero struct {
	Mode                     ScaleToZeroMode                `json:"mode,omitempty"`
	ScalerInterval           string                         `json:"scalerInterval,omitempty"`
	ResourceReadinessTimeout string                         `json:"resourceReadinessTimeout,omitempty"`
	ScaleResources           []functionconfig.ScaleResource `json:"scaleResources,omitempty"`
	InactivityWindowPresets  []string                       `json:"inactivityWindowPresets,omitempty"`

	// Used to enrich special scale-to-zero ingress annotations
	HTTPTriggerIngressAnnotations map[string]string `json:"httpTriggerIngressAnnotations,omitempty"`

	// Used for DLX options, selects in which way to send invocation when multiple targets are given:
	// random, primary or canary.
	MultiTargetStrategy scalertypes.MultiTargetStrategy `json:"multiTargetStrategy,omitempty"`
}

type ScaleToZeroMode string

const (
	EnabledScaleToZeroMode  ScaleToZeroMode = "enabled"
	DisabledScaleToZeroMode ScaleToZeroMode = "disabled"
)

type AutoScale struct {
	MetricName  string `json:"metricName,omitempty"`
	TargetValue string `json:"targetValue,omitempty"`
}

type Metrics struct {
	Sinks     map[string]MetricSink `json:"sinks,omitempty"`
	System    []string              `json:"system,omitempty"`
	Functions []string              `json:"functions,omitempty"`
}

type LabelSelectorAndConfig struct {
	LabelSelector  machinarymetav1.LabelSelector `json:"labelSelector,omitempty"`
	FunctionConfig functionconfig.Config         `json:"functionConfig,omitempty"`
	Kubernetes     Kubernetes                    `json:"kubernetes,omitempty"`
}

type Kubernetes struct {
	Deployment *appsv1.Deployment `json:"deployment,omitempty"`
}

type ProjectsLeaderKind string

const (
	ProjectsLeaderKindIguazio ProjectsLeaderKind = "iguazio"
	ProjectsLeaderKindMlrun   ProjectsLeaderKind = "mlrun"
	ProjectsLeaderKindMock    ProjectsLeaderKind = "mock"
)

type ProjectsLeader struct {
	Kind                    ProjectsLeaderKind `json:"kind,omitempty"`
	APIAddress              string             `json:"apiAddress,omitempty"`
	SynchronizationInterval string             `json:"synchronizationInterval,omitempty"`
}

type PlatformKubeConfig struct {
	KubeConfigPath string `json:"kubeConfigPath,omitempty"`

	// TODO: Move IngressConfig here
	DefaultServiceType               corev1.ServiceType  `json:"defaultServiceType,omitempty"`
	DefaultFunctionNodeSelector      map[string]string   `json:"defaultFunctionNodeSelector,omitempty"`
	DefaultFunctionTolerations       []corev1.Toleration `json:"defaultFunctionTolerations,omitempty"`
	DefaultHTTPIngressHostTemplate   string              `json:"defaultHTTPIngressHostTemplate,omitempty"`
	DefaultHTTPIngressAnnotations    map[string]string   `json:"defaultHTTPIngressAnnotations,omitempty"`
	DefaultFunctionPriorityClassName string              `json:"defaultFunctionPriorityClassName,omitempty"`
	ValidFunctionPriorityClassNames  []string            `json:"validFunctionPriorityClassNames,omitempty"`
}

type PlatformLocalConfig struct {
	FunctionContainersHealthinessEnabled  bool
	FunctionContainersHealthinessTimeout  time.Duration
	FunctionContainersHealthinessInterval time.Duration
}

type ImageRegistryOverridesConfig struct {

	// maps are [runtime -> registry]
	BaseImageRegistries    map[string]string `json:"baseImageRegistries,omitempty"`
	OnbuildImageRegistries map[string]string `json:"onbuildImageRegistries,omitempty"`
}

// IngressConfig holds the default values for created ingresses
type IngressConfig struct {
	EnableSSLRedirect          bool     `json:"enableSSLRedirect,omitempty"`
	TLSSecret                  string   `json:"tlsSecret,omitempty"`
	IguazioAuthURL             string   `json:"iguazioAuthURL,omitempty"`
	IguazioSignInURL           string   `json:"iguazioSignInURL,omitempty"`
	AllowedAuthenticationModes []string `json:"allowedAuthenticationModes,omitempty"`
	Oauth2ProxyURL             string   `json:"oauth2ProxyURL,omitempty"`
}

type CronTriggerCreationMode string

const (
	ProcessorCronTriggerCreationMode CronTriggerCreationMode = "processor"
	KubeCronTriggerCreationMode      CronTriggerCreationMode = "kube"

	DefaultServiceType = corev1.ServiceTypeClusterIP
)
