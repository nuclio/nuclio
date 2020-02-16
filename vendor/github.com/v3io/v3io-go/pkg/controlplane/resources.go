/*
Copyright 2018 The v3io Authors.

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

package v3ioc

type SessionAttributes struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Plane         string `json:"plane,omitempty"`
	InterfaceType string `json:"interfaceType,omitempty"`
}

type UserAttributes struct {
	AssignedPolicies       []string `json:"assigned_policies,omitempty"`
	AuthenticationScheme   string   `json:"authentication_scheme,omitempty"`
	CreatedAt              string   `json:"created_at,omitempty"`
	DataAccessMode         string   `json:"data_access_mode,omitempty"`
	Department             string   `json:"department,omitempty"`
	Description            string   `json:"description,omitempty"`
	Email                  string   `json:"email,omitempty"`
	Enabled                bool     `json:"enabled,omitempty"`
	FirstName              string   `json:"first_name,omitempty"`
	JobTitle               string   `json:"job_title,omitempty"`
	LastName               string   `json:"last_name,omitempty"`
	PasswordChangedAt      string   `json:"password_changed_at,omitempty"`
	PhoneNumber            string   `json:"phone_number,omitempty"`
	SendPasswordOnCreation bool     `json:"send_password_on_creation,omitempty"`
	UID                    int      `json:"uid,omitempty"`
	UpdatedAt              string   `json:"updated_at,omitempty"`
	Username               string   `json:"username,omitempty"`
	Password               string   `json:"password,omitempty"`
}

type ContainerAttributes struct {
	Name string `json:"name,omitempty"`
}

type ClusterInfoAttributes struct {
	Endpoints ClusterInfoEndpoints `json:"endpoints,omitempty"`
}

type ClusterInfoEndpoints struct {
	AppServices []ClusterInfoEndpointAddress `json:"app_services,omitempty"`
}

type ClusterInfoEndpointAddress struct {
	ServiceID   string   `json:"service_id,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version,omitempty"`
	Urls        []string `json:"urls,omitempty"`
	APIUrls     []string `json:"api_urls,omitempty"`
}

type EventAttributes struct {
	SystemEvent    bool           `json:"system_event,omitempty"`
	Source         string         `json:"source,omitempty"`
	Kind           string         `json:"kind,omitempty"`
	Description    string         `json:"description,omitempty"`
	Severity       Severity       `json:"severity,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
	Visibility     Visibility     `json:"visibility,omitempty"`
	Classification Classification `json:"classification,omitempty"`

	TimestampUint64  uint64 `json:"timestamp_uint64,omitempty"`
	TimestampIso8601 string `json:"timestamp_iso8601,omitempty"`

	ParametersUint64 []ParameterUint64 `json:"parameters_uint64,omitempty"`
	ParametersText   []ParameterText   `json:"parameters_text,omitempty"`

	InvokingUserID string `json:"invoking_user_id,omitempty"`
	AuditTenant    string `json:"audit_tenant,omitempty"`
}

type AffectedResource struct {
	ResourceType string `json:"resource_type,omitempty"`
	IDStr        string `json:"id_str,omitempty"`
	ID           uint64 `json:"id,omitempty"`
	Name         string `json:"name,omitempty"`
}

type ParameterText struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type ParameterUint64 struct {
	Name  string `json:"name,omitempty"`
	Value uint64 `json:"value,omitempty"`
}

type Severity string

const (
	UnknownSeverity  Severity = "unknown"
	DebugSeverity    Severity = "debug"
	InfoSeverity     Severity = "info"
	WarningSeverity  Severity = "warning"
	MajorSeverity    Severity = "major"
	CriticalSeverity Severity = "critical"
)

type Visibility string

const (
	UnknownVisibility      Visibility = "unknown"
	InternalVisibility     Severity   = "internal"
	ExternalVisibility     Severity   = "external"
	CustomerOnlyVisibility Severity   = "CustomerOnly"
)

type Classification string

const (
	UnknownClassification Classification = "unknown"
	HwClassification      Classification = "hw"
	UaClassification      Classification = "ua"
	BgClassification      Classification = "bg"
	SwClassification      Classification = "sw"
	SLAClassification     Classification = "sla"
	CapClassification     Classification = "cap"
	SecClassification     Classification = "sec"
	AuditClassification   Classification = "audit"
	SystemClassification  Classification = "system"
)

// AccessKey holds info about a access key
type AccessKeyAttributes struct {
	TTL           int      `json:"ttl,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
	ExpiresAt     int      `json:"expires_at,omitempty"`
	GroupIds      []string `json:"group_ids,omitempty"`
	UID           int      `json:"uid,omitempty"`
	GIDs          []int    `json:"gids,omitempty"`
	TenantID      string   `json:"tenant_id,omitempty"`
	Kind          string   `json:"kind,omitempty"`
	Plane         Plane    `json:"plane,omitempty"`
	InterfaceKind string   `json:"interface_kind,omitempty"`
	Label         string   `json:"label,omitempty"`
}

type Plane string

const (
	ControlPlane Plane = "control"
	DataPlane    Plane = "data"
)

type Kind string

const (
	SessionKind   Kind = "session"
	AccessKeyKind Kind = "accessKey"
)
