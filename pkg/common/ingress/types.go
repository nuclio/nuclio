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
	HtpasswdContents     string
	WhitelistIPAddresses []string
	AllowedProtocols     []string
	SSLPassthrough       bool
	AllowSSLRedirect     bool
	BackendProtocol      string
	TLSSecret            string
	RewriteTarget        string
	UpstreamVhost        string
	ProxyReadTimeout     string
	Annotations          map[string]AnnotationValue
}

type SpecRole string

type AnnotationValue struct {
	Value string

	// By default every annotation value is being quoted by simply surrounding it with double quotes
	// It is done since annotation in kubernetes is map[string]string object, so if the value will be 3 (or true)
	// yaml.Unmarshal will parse it as a number (or boolean), which makes kubernetes to ignore it
	// If the annotation value is a string that includes double quotes it won't work, since the double quotes should be
	// escaped. So in that case this parameter should be set to true, so that the value will be wrapped with quotes
	// using common.Quote()
	QuoteEscapingNeeded bool
}

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
