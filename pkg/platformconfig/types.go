/*
Copyright 2023 The Nuclio Authors.

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

	auth "github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"

	nucliozap "github.com/nuclio/zap"
	"github.com/v3io/scaler/pkg/scalertypes"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	machinarymetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultFunctionReadinessTimeoutSeconds  = 120
	DefaultFunctionInvocationTimeoutSeconds = 60
	DefaultReadinessPollInterval            = 3 * time.Second
)

var DefaultReadinessProbeConfiguration = &corev1.Probe{
	InitialDelaySeconds: int32(5),
	TimeoutSeconds:      int32(1),
	PeriodSeconds:       int32(1),
	FailureThreshold:    int32(10),
}

var DefaultLivenessProbeConfiguration = &corev1.Probe{
	InitialDelaySeconds: int32(10),
	TimeoutSeconds:      int32(3),
	PeriodSeconds:       int32(5),
	FailureThreshold:    int32(3),
}

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

	// Used for DLX options, specifies how often the DLX resync it's internal state
	ResyncInterval string `json:"resyncInterval,omitempty"`

	// Used by the resource scaler, how often to poll the function CRD state while
	// waiting for it to become ready on scale-from-zero. Defaults to DefaultReadinessPollInterval.
	ReadinessPollInterval string `json:"readinessPollInterval,omitempty"`

	// Used for scaler options, specifies metrics client configuration and type
	MetricsClient MetricsClientConfig `json:"metricsClient,omitempty"`
}

type ScaleToZeroMode string

type MetricsTemplate struct {
	Name     string `json:"name,omitempty"`
	Template string `json:"template,omitempty"`
}

type MetricsClientConfig struct {
	Kind      scalertypes.MetricsClientKind `json:"kind,omitempty"`
	URL       string                        `json:"url,omitempty"`
	Templates []MetricsTemplate             `json:"templates,omitempty"`
}

const (
	EnabledScaleToZeroMode  ScaleToZeroMode = "enabled"
	DisabledScaleToZeroMode ScaleToZeroMode = "disabled"
)

type AutoScaleMetricsMode string

const (

	// AutoScaleMetricsModeLegacy is the legacy mode, where CPU usage is used for scaling
	AutoScaleMetricsModeLegacy AutoScaleMetricsMode = "legacy"

	// AutoScaleMetricsModeCustom uses custom metrics for scaling
	AutoScaleMetricsModeCustom AutoScaleMetricsMode = "custom"
)

func AutoScaleMetricsModeIsValid(autoScaleMode AutoScaleMetricsMode) bool {
	for _, mode := range []AutoScaleMetricsMode{
		AutoScaleMetricsModeLegacy,
		AutoScaleMetricsModeCustom,
	} {
		if autoScaleMode == mode {
			return true
		}
	}
	return false
}

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
	ProjectsLeaderKindOris    ProjectsLeaderKind = "oris"
	ProjectsLeaderKindMock    ProjectsLeaderKind = "mock"
)

const (
	// DefaultProjectSync2PCEnabled is false: 2PC validation is opt-in so that deployments
	// running a pre-2PC MLRun version continue to work without any configuration change.
	DefaultProjectSync2PCEnabled = false
)

type ProjectsLeader struct {
	Kind                    ProjectsLeaderKind `json:"kind,omitempty"`
	APIAddress              string             `json:"apiAddress,omitempty"`
	SynchronizationInterval string             `json:"synchronizationInterval,omitempty"`

	// ProjectSync2PCEnabled enables two-phase-commit validation for leader-origin project requests.
	// When false (default), all leader requests bypass 2PC checks and are applied unconditionally,
	// preserving backwards compatibility with pre-2PC MLRun versions.
	ProjectSync2PCEnabled bool `json:"projectSync2PCEnabled,omitempty"`

	// SyncOnStartup, when true, performs a single project sync from the leader on startup instead of (or in
	// addition to) the periodic interval loop.
	SyncOnStartup bool `json:"syncOnStartup,omitempty"`

	// Identity is the authenticated username the leader is expected to present on leader-origin calls.
	Identity string `json:"identity,omitempty"`
}

// TrustsLeaderOrigin reports whether a caller presenting the given session should be trusted as
// leader-origin. Kinds other than ProjectsLeaderKindOris keep trusting the legacy X-Projects-Role header unconditionally.
// Oris requires session.GetUsername() to match Leader Identity. A nil receiver, nil session, or empty Leader dentity all fail closed.
func (pl *ProjectsLeader) TrustsLeaderOrigin(session auth.Session) bool {
	if pl == nil || pl.Kind != ProjectsLeaderKindOris {
		return true
	}
	return pl.Identity != "" && session != nil && session.GetUsername() == pl.Identity
}

type PlatformKubeConfig struct {
	KubeConfigPath string `json:"kubeConfigPath,omitempty"`

	// TODO: Move IngressConfig here
	DefaultServiceType                       corev1.ServiceType      `json:"defaultServiceType,omitempty"`
	DefaultFunctionNodeSelector              map[string]string       `json:"defaultFunctionNodeSelector,omitempty"`
	DefaultHTTPIngressHostTemplate           string                  `json:"defaultHTTPIngressHostTemplate,omitempty"`
	DefaultHTTPIngressAnnotations            map[string]string       `json:"defaultHTTPIngressAnnotations,omitempty"`
	DefaultHTTPIngressClassName              string                  `json:"defaultHTTPIngressClassName,omitempty"`
	DefaultFunctionPriorityClassName         string                  `json:"defaultFunctionPriorityClassName,omitempty"`
	DefaultFunctionServiceAccount            string                  `json:"defaultFunctionServiceAccount,omitempty"`
	DefaultForbiddenServiceAccounts          []string                `json:"defaultForbiddenServiceAccounts,omitempty"`
	ValidFunctionPriorityClassNames          []string                `json:"validFunctionPriorityClassNames,omitempty"`
	DefaultFunctionPodResources              PodResourceRequirements `json:"defaultFunctionPodResources,omitempty"`
	DefaultSidecarResources                  PodResourceRequirements `json:"defaultSidecarResources,omitempty"`
	DefaultFunctionTolerations               []corev1.Toleration     `json:"defaultFunctionTolerations,omitempty"`
	PreemptibleNodes                         *PreemptibleNodes       `json:"preemptibleNodes,omitempty"`
	DefaultReadinessProbe                    *corev1.Probe           `json:"readinessProbe,omitempty"`
	DefaultLivenessProbe                     *corev1.Probe           `json:"livenessProbe,omitempty"`
	ElasticSearchConfig                      *ElasticSearchConfig    `json:"elasticSearchConfig,omitempty"`
	ProjectSecretTemplate                    string                  `json:"projectSecretTemplate,omitempty"`
	ProjectSecretAllowedServiceAccountsKey   string                  `json:"projectSecretAllowedServiceAccountsKey,omitempty"`
	ProjectSecretForbiddenServiceAccountsKey string                  `json:"projectSecretForbiddenServiceAccountsKey,omitempty"`
	ProjectSecretDefaultServiceAccountKey    string                  `json:"projectSecretDefaultServiceAccountKey,omitempty"`
}

// IsConfiguredToVerifyServiceAccount checks if the platform kube config is configured to verify service accounts
func (pkc *PlatformKubeConfig) IsConfiguredToVerifyServiceAccount() bool {
	if len(pkc.DefaultForbiddenServiceAccounts) > 0 {
		return true
	}
	// if project secret template is not specified, skip validation
	if pkc.ProjectSecretTemplate == "" {
		return false
	}

	// if project secret service accounts keys are not configured, skip validation
	if pkc.ProjectSecretAllowedServiceAccountsKey == "" &&
		pkc.ProjectSecretForbiddenServiceAccountsKey == "" {
		return false
	}
	return true
}

// IsConfiguredToEnrichServiceAccount checks if the platform kube config is configured to enrich service accounts
func (pkc *PlatformKubeConfig) IsConfiguredToEnrichServiceAccount() bool {
	if pkc.DefaultFunctionServiceAccount != "" {
		// if default function service account is set, should enrich
		return true
	}

	// if project secret template is not specified, skip validation
	if pkc.ProjectSecretTemplate == "" {
		return false
	}

	// if project secret allowed service accounts key is not configured, skip validation
	if pkc.ProjectSecretDefaultServiceAccountKey == "" {
		return false
	}
	return true
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
	FunctionContainersHealthinessEnabled         bool                        `json:"functionContainersHealthinessEnabled"`
	FunctionContainersHealthinessTimeout         time.Duration               `json:"functionContainersHealthinessTimeout,omitempty"`
	FunctionContainersHealthinessInterval        time.Duration               `json:"functionContainersHealthinessInterval,omitempty"`
	FunctionContainersGracefulTerminationTimeout time.Duration               `json:"functionContainersGracefulTerminationTimeout,omitempty"`
	DefaultFunctionContainerNetworkName          string                      `json:"defaultFunctionContainerNetworkName,omitempty"`
	DefaultFunctionRestartPolicy                 *dockerclient.RestartPolicy `json:"defaultFunctionRestartPolicy,omitempty"`
	DefaultFunctionVolumes                       []functionconfig.Volume     `json:"defaultFunctionVolumes,omitempty"`
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

// ServiceAccountConfig holds configuration for service account tokens
type ServiceAccountConfig struct {
	Enabled                bool    `json:"enabled,omitempty"`
	TokenPath              string  `json:"tokenPath,omitempty"`
	TokenExpirationSeconds int     `json:"tokenExpirationSeconds,omitempty"`
	TokenRefreshRatio      float64 `json:"tokenRefreshRatio,omitempty"`
}

// LogProxyKind represents the type of log proxy backend (elasticsearch or opensearch)
type LogProxyKind string

const (
	// LogProxyKindElasticSearch indicates an Elasticsearch backend
	LogProxyKindElasticSearch LogProxyKind = "elasticsearch"
	// LogProxyKindOpenSearch indicates an OpenSearch backend
	LogProxyKindOpenSearch LogProxyKind = "opensearch"
)

type ElasticSearchConfig struct {
	URL                  string `json:"url,omitempty"`
	SSLVerificationMode  string `json:"sslVerificationMode,omitempty"`
	Username             string `json:"username,omitempty"`
	Password             string `json:"password,omitempty"`
	APIKey               string `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	Index                string `json:"index,omitempty"`
	CustomQueryParameter string `json:"customQueryParameter,omitempty"`

	// Kind specifies the log proxy backend type explicitly.
	// If not set, the backend type is auto-detected by querying the search engine.
	// Valid values: "elasticsearch", "opensearch"
	Kind LogProxyKind `json:"kind,omitempty"`
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
	DefaultHTTPIngressClassName      = "nginx"
)

type StreamMonitoringConfig struct {
	WebapiURL              string `json:"webapiURL,omitempty"`
	V3ioRequestConcurrency uint   `json:"v3ioRequestConcurrency,omitempty"`
}

type SensitiveFieldPath string

type SensitiveFieldsConfig struct {

	// CustomSensitiveFields is a list of fields that should be masked in logs and function config
	MaskSensitiveFields   bool             `json:"maskSensitiveFields,omitempty"`
	CustomSensitiveFields []string         `json:"customSensitiveFields,omitempty"`
	SensitiveFieldsRegex  []*regexp.Regexp `json:"sensitiveFieldsRegex,omitempty"`
}

func (sfc *SensitiveFieldsConfig) GetDefaultSensitiveFields() []string {
	return []string{

		// build
		"^/spec/build/codeentryattributes/password$",
		"^/spec/build/codeentryattributes/s3secretaccesskey$",
		"^/spec/build/codeentryattributes/s3sessiontoken$",
		"^/spec/build/codeentryattributes/headers/authorization$",
		"^/spec/build/codeentryattributes/headers/x-v3io-session-key$",

		// volumes
		"^/spec/volumes\\[\\d+\\]/volume/volumesource/flexvolume/options/accesskey$",
		"^/spec/volumes\\[\\d+\\]/volume/flexvolume/options/accesskey$",

		// triggers - global
		"^/spec/triggers/.+/password$",
		"^/spec/triggers/.+/secret$",
		// triggers - specific
		// - v3io stream
		"^/spec/triggers/.+/attributes/password$",
		// - kinesis
		"^/spec/triggers/.+/attributes/accesskeyid$",
		"^/spec/triggers/.+/attributes/secretaccesskey$",
		// - kafka
		"^/spec/triggers/.+/attributes/cacert$",
		"^/spec/triggers/.+/attributes/accesskey$",
		"^/spec/triggers/.+/attributes/accesscertificate$",
		"^/spec/triggers/.+/attributes/sasl/password$",
		"^/spec/triggers/.+/attributes/sasl/oauth/clientsecret$",
		// - http function-level basic auth
		"^/spec/triggers/.+/attributes/authentication/basicauth/password$",
		// - kafka annotations
		"^/metadata/annotations/nuclio\\.io/kafka-ca-cert$",
		"^/metadata/annotations/nuclio\\.io/kafka-access-key$",
		"^/metadata/annotations/nuclio\\.io/kafka-access-cert$",
		"^/metadata/annotations/nuclio\\.io/kafka-sasl-password$",
		"^/metadata/annotations/nuclio\\.io/kafka-sasl-oauth-client-secret$",
		"^/metadata/annotations/nuclio\\.io/kafka-sasl-oauth-token-url$",
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

// Authentication holds authentication-related platform configuration.
type Authentication struct {

	// FunctionAuthenticationEnabled gates behind-Service function-level authentication.
	// When false (default) functions keep ingress-level authentication.
	FunctionAuthenticationEnabled bool `json:"functionAuthenticationEnabled,omitempty"`

	// AuthURL is the back-end auth-check endpoint URL.
	AuthURL string `json:"authURL,omitempty"`

	// SignInURL is the sign-in redirect URL.
	SignInURL string `json:"signInURL,omitempty"`

	// AuthKind is the auth client kind used by the auth-proxy sidecar for API/browser authentication.
	// Defaults to the value set in the OPA config (Opa.AuthKind).
	AuthKind auth.Kind `json:"authKind,omitempty"`

	// AllowedModes lists the function-level authentication modes the platform permits.
	AllowedModes []string `json:"allowedAuthenticationModes,omitempty"`

	// DefaultMode is the platform-wide default function-level authentication mode
	// stamped onto an HTTP trigger when it does not set one explicitly.
	DefaultMode auth.AuthenticationMode `json:"defaultAuthenticationMode,omitempty"`
}

// GetAllowedFunctionAuthenticationModes returns the allowed function-level authentication modes.
// Returns AllowedModes when configured; otherwise the built-in default set.
func (a *Authentication) GetAllowedFunctionAuthenticationModes() []string {
	if len(a.AllowedModes) > 0 {
		return a.AllowedModes
	}
	return []string{
		string(auth.AuthenticationModeNone),
		string(auth.AuthenticationModeAPI),
		string(auth.AuthenticationModeBrowser),
		string(auth.AuthenticationModeBasicAuth),
	}
}
