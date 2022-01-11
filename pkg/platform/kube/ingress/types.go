package ingress

import networkingv1 "k8s.io/api/networking/v1"

type Spec struct {
	Name                 string
	Namespace            string
	ProjectName          string
	APIGatewayName       string
	Host                 string
	Path                 string
	PathType             *networkingv1.PathType
	ServiceName          string
	ServicePort          int
	AuthenticationMode   AuthenticationMode
	Authentication       *Authentication
	WhitelistIPAddresses []string
	SSLPassthrough       bool
	EnableSSLRedirect    *bool
	BackendProtocol      string
	TLSSecret            string
	RewriteTarget        string
	UpstreamVhost        string
	ProxyReadTimeout     string
	Annotations          map[string]string
}

type SpecRole string

type Authentication struct {
	BasicAuth *BasicAuth `json:"basicAuth,omitempty"`
	DexAuth   *DexAuth   `json:"dexAuth,omitempty"`
}

type BasicAuth struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

type DexAuth struct {
	Oauth2ProxyURL               string `json:"oauth2ProxyUrl,omitempty"`
	RedirectUnauthorizedToSignIn bool   `json:"redirectUnauthorizedToSignIn,omitempty"`
}

type AuthenticationMode string

const (
	AuthenticationModeNone      AuthenticationMode = "none"
	AuthenticationModeBasicAuth AuthenticationMode = "basicAuth"
	AuthenticationModeAccessKey AuthenticationMode = "accessKey"
	AuthenticationModeOauth2    AuthenticationMode = "oauth2"
)
