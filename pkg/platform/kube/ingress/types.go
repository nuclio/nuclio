package ingress

type Spec struct {
	Name                 string
	Namespace            string
	Host                 string
	Path                 string
	ServiceName          string
	ServicePort          int
	AuthenticationMode   AuthenticationMode
	Authentication       *Authentication
	WhitelistIPAddresses []string
	AllowedProtocols     []string
	SSLPassthrough       bool
	AllowSSLRedirect     bool
	BackendProtocol      string
	TLSSecret            string
	RewriteTarget        string
	UpstreamVhost        string
	ProxyReadTimeout     string
	Annotations          map[string]string
}

type SpecRole string

type Authentication struct {
	BasicAuth *BasicAuth `json:"basic_auth,omitempty"`
	DexAuth   *DexAuth   `json:"dex_auth,omitempty"`
}

type BasicAuth struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

type DexAuth struct {
	Oauth2ProxyURL string `json:"oauth2_proxy_url,omitempty"`
}

type AuthenticationMode string

const (
	AuthenticationModeNone                               AuthenticationMode = "none"
	AuthenticationModeBasicAuth                          AuthenticationMode = "basicAuth"
	AuthenticationModeAccessKey                          AuthenticationMode = "data"
	AuthenticationModeDex                                AuthenticationMode = "dex"
)
