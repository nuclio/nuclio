package platformconfig

type LoggerSink struct {
	Driver     string                 `json:"driver,omitempty"`
	URL        string                 `json:"url,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type SystemLogger struct {
	Level string `json:"level,omitempty"`
	Sink  string `json:"sink,omitempty"`
}

type FunctionsLogger struct {
	DefaultLevel string `json:"defaultLevel,omitempty"`
	DefaultSink  string `json:"defaultSink,omitempty"`
}

type Logger struct {
	Sinks     map[string]LoggerSink `json:"sinks,omitempty"`
	System    SystemLogger          `json:"system,omitempty"`
	Functions FunctionsLogger       `json:"functions,omitempty"`
}

type WebAdmin struct {
	Enabled       bool   `json:"enabled,omitempty"`
	ListenAddress string `json:"listenAddress,omitempty"`
}

type MetricSink struct {
	Driver     string                 `json:"driver,omitempty"`
	URL        string                 `json:"url,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type Metrics struct {
	Sinks       map[string]MetricSink `json:"sinks,omitempty"`
	Enabled     bool                  `json:"enabled,omitempty"`
	DefaultSink string                `json:"defaultSink,omitempty"`
}

type Config struct {
	WebAdmin WebAdmin `json:"webAdmin,omitempty"`
	Logger   Logger   `json:"logger,omitempty"`
	Metrics  Metrics  `json:"metrics,omitempty"`
}
