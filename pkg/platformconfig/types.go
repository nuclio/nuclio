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
	"regexp"
	"sort"
	"time"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"

	nucliozap "github.com/nuclio/zap"
	"github.com/v3io/scaler/pkg/scalertypes"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	machinarymetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const DefaultFunctionReadinessTimeoutSeconds = 120

type LoggerSinkKind string

const (
	LoggerSinkKindStdout      LoggerSinkKind = "stdout"
	LoggerSinkKindAppInsights LoggerSinkKind = "appinsights"

	// LoggerSinkKindElasticsearch is not supported
	LoggerSinkKindElasticsearch LoggerSinkKind = "elasticsearch"
)

type LoggerSink struct {
	Kind       LoggerSinkKind         `json:"kind,omitempty"`
	URL        string                 `json:"url,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type LoggerSinkWithLevel struct {
	Level string
	Sink  LoggerSink

	redactor *nucliozap.Redactor
}

func (l *LoggerSinkWithLevel) GetRedactingLogger() *nucliozap.Redactor {
	return l.redactor
}

type LoggerSinkBinding struct {
	Level string `json:"level,omitempty"`
	Sink  string `json:"sink,omitempty"`
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
	DefaultServiceType               corev1.ServiceType      `json:"defaultServiceType,omitempty"`
	DefaultFunctionNodeSelector      map[string]string       `json:"defaultFunctionNodeSelector,omitempty"`
	DefaultHTTPIngressHostTemplate   string                  `json:"defaultHTTPIngressHostTemplate,omitempty"`
	DefaultHTTPIngressAnnotations    map[string]string       `json:"defaultHTTPIngressAnnotations,omitempty"`
	DefaultFunctionPriorityClassName string                  `json:"defaultFunctionPriorityClassName,omitempty"`
	DefaultFunctionServiceAccount    string                  `json:"defaultFunctionServiceAccount,omitempty"`
	ValidFunctionPriorityClassNames  []string                `json:"validFunctionPriorityClassNames,omitempty"`
	DefaultFunctionPodResources      PodResourceRequirements `json:"defaultFunctionPodResources,omitempty"`
	DefaultFunctionTolerations       []corev1.Toleration     `json:"defaultFunctionTolerations,omitempty"`
	PreemptibleNodes                 *PreemptibleNodes       `json:"preemptibleNodes,omitempty"`
}

// PreemptibleNodes Holds data needed when user decided to run his function pods on a preemptible node (aka Spot node)
type PreemptibleNodes struct {
	DefaultMode    functionconfig.RunOnPreemptibleNodeMode `json:"defaultMode,omitempty"`
	Tolerations    []corev1.Toleration                     `json:"tolerations,omitempty"`
	GPUTolerations []corev1.Toleration                     `json:"gpuTolerations,omitempty"`
	NodeSelector   map[string]string                       `json:"nodeSelector,omitempty"`
}

// CompileAffinityByLabelSelector compiles affinity spec based on pre-configured node selector
func (p *PreemptibleNodes) CompileAffinityByLabelSelector(
	operation corev1.NodeSelectorOperator) []corev1.NodeSelectorRequirement {
	var matchExpressions []corev1.NodeSelectorRequirement
	for nodeSelectorKey, nodeSelectorValue := range p.NodeSelector {
		matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
			Key:      nodeSelectorKey,
			Operator: operation,
			Values:   []string{nodeSelectorValue},
		})
	}

	//make compilation deterministic
	sort.Slice(matchExpressions, func(i, j int) bool {
		return matchExpressions[i].String() < matchExpressions[j].String()
	})
	return matchExpressions
}

// CompileAffinityByLabelSelectorScheduleOnOneOfMatchingNodes schedule on a node having at least one of the node selectors (ORed)
func (p *PreemptibleNodes) CompileAffinityByLabelSelectorScheduleOnOneOfMatchingNodes() []corev1.NodeSelectorTerm {
	affinity := p.CompileAffinityByLabelSelector(corev1.NodeSelectorOpIn)
	var nodeSelectorTerms []corev1.NodeSelectorTerm
	for _, expression := range affinity {
		nodeSelectorTerms = append(nodeSelectorTerms, corev1.NodeSelectorTerm{
			MatchExpressions: []corev1.NodeSelectorRequirement{expression},
		})
	}
	return nodeSelectorTerms
}

func (p *PreemptibleNodes) CompileAntiAffinityByLabelSelectorNoScheduleOnMatchingNodes() []corev1.NodeSelectorTerm {
	antiAffinity := p.CompileAffinityByLabelSelector(corev1.NodeSelectorOpNotIn)

	// using a single term with potentially multiple expressions to ensure anti affinity.
	// when having multiple terms, pod scheduling is succeeded if at least one
	// term is satisfied.
	return []corev1.NodeSelectorTerm{
		{
			MatchExpressions: antiAffinity,
		},
	}
}

type PlatformLocalConfig struct {
	FunctionContainersHealthinessEnabled  bool                        `json:"functionContainersHealthinessEnabled"`
	FunctionContainersHealthinessTimeout  time.Duration               `json:"functionContainersHealthinessTimeout,omitempty"`
	FunctionContainersHealthinessInterval time.Duration               `json:"functionContainersHealthinessInterval,omitempty"`
	DefaultFunctionContainerNetworkName   string                      `json:"defaultFunctionContainerNetworkName,omitempty"`
	DefaultFunctionRestartPolicy          *dockerclient.RestartPolicy `json:"defaultFunctionRestartPolicy,omitempty"`
	DefaultFunctionVolumes                []functionconfig.Volume     `json:"defaultFunctionVolumes,omitempty"`
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

type PodResourceRequirements struct {
	Requests ResourceRequirements `json:"requests,omitempty"`
	Limits   ResourceRequirements `json:"limits,omitempty"`
}

type ResourceRequirements struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

const (
	DefaultStreamMonitoringWebapiURL = "http://v3io-webapi:8081"
	DefaultV3ioRequestConcurrency    = 64
)

type StreamMonitoringConfig struct {
	WebapiURL              string `json:"webapiURL,omitempty"`
	V3ioRequestConcurrency uint   `json:"v3ioRequestConcurrency,omitempty"`
}

type SensitiveFieldPath string

type SensitiveFieldsConfig struct {

	// CustomSensitiveFields is a list of fields that should be masked in logs and function config
	MaskSensitiveFields   bool     `json:"maskSensitiveFields,omitempty"`
	CustomSensitiveFields []string `json:"sensitiveFields,omitempty"`
	SensitiveFieldsRegex  []*regexp.Regexp
}

func (sfc *SensitiveFieldsConfig) GetDefaultSensitiveFields() []string {
	return []string{

		// build
		"^/spec/build/codeentryattributes/password",
		// volumes
		"^/spec/volumes\\[\\d+\\]/volume/volumesource/flexvolume/options/accesskey",
		"^/spec/volumes\\[\\d+\\]/volume/flexvolume/options/accesskey",

		// triggers - global
		"^/spec/triggers/.+/password",
		"^/spec/triggers/.+/secret",
		// triggers - specific
		// - v3io stream
		"^/spec/triggers/.+/attributes/password",
		// - kinesis
		"^/spec/triggers/.+/attributes/accesskeyid",
		"^/spec/triggers/.+/attributes/secretaccesskey",
		// - kafka
		"^/spec/triggers/.+/attributes/cacert",
		"^/spec/triggers/.+/attributes/accesskey",
		"^/spec/triggers/.+/attributes/accesscertificate",
		"^/spec/triggers/.+/attributes/sasl/password",
		"^/spec/triggers/.+/attributes/sasl/oauth/clientsecret",
	}
}

func (sfc *SensitiveFieldsConfig) GetSensitiveFields() []string {
	return append(sfc.CustomSensitiveFields, sfc.GetDefaultSensitiveFields()...)
}

func (sfc *SensitiveFieldsConfig) CompileSensitiveFieldsRegex() []*regexp.Regexp {
	if sfc.SensitiveFieldsRegex == nil {
		for _, field := range sfc.GetSensitiveFields() {

			// compile each regular expression as case-insensitive
			sfc.SensitiveFieldsRegex = append(sfc.SensitiveFieldsRegex, regexp.MustCompile("(?i)"+field))
		}
	}
	return sfc.SensitiveFieldsRegex
}
