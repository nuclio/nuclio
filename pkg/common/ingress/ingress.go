package ingress

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/nuclio/nuclio/pkg/common"
	"strings"

	"github.com/nuclio/errors"
)

func GenerateIngressResource(ctx context.Context,
	spec Spec) (string, error) {

	var err error
	var secretResourceContents string
	ingressAnnotations := map[string]AnnotationValue{
		"kubernetes.io/ingress.class": {Value: "nginx"},
	}
	if spec.RewriteTarget != "" {
		ingressAnnotations["nginx.ingress.kubernetes.io/rewrite-target"] = AnnotationValue{Value: spec.RewriteTarget}
	}

	if spec.UpstreamVhost != "" {
		ingressAnnotations["nginx.ingress.kubernetes.io/upstream-vhost"] = AnnotationValue{Value: spec.UpstreamVhost}
	}

	if spec.BackendProtocol != "" {
		ingressAnnotations["nginx.ingress.kubernetes.io/backend-protocol"] =
			AnnotationValue{Value: spec.BackendProtocol}
	}

	if spec.SSLPassthrough {
		ingressAnnotations["nginx.ingress.kubernetes.io/ssl-passthrough"] = AnnotationValue{Value: "true"}
	} else {
		var authIngressAnnotations map[string]AnnotationValue

		switch spec.AuthenticationMode {
		case AuthenticationModeNone:
			//do nothing
		case AuthenticationModeBasicAuth:
			authIngressAnnotations, secretResourceContents, err = getBasicAuthIngressAnnotationsAndSecret(ctx, spec)
			if err != nil {
				return "", errors.Wrap(err, "Failed to get basic auth annotations")
			}
		case AuthenticationModeAccessKey:
			authIngressAnnotations, err = getSessionVerificationAnnotations("/api/data_sessions/verifications")
			if err != nil {
				return "", errors.Wrap(err, "Failed to get access key auth mode annotations")
			}
		case AuthenticationModeDex:
			authIngressAnnotations, err = getDexAuthIngressAnnotations(spec)
			if err != nil {
				return "", errors.Wrap(err, "Failed to get dex auth annotations")
			}
		default:
			return "", errors.Errorf("Unknown ingress authentication mode: %s", spec.AuthenticationMode)
		}

		// merge with existing annotation map
		for annotation, annotationValue := range authIngressAnnotations {
			ingressAnnotations[annotation] = annotationValue
		}

		ingressAnnotations["nginx.ingress.kubernetes.io/proxy-body-size"] = AnnotationValue{Value: "0"}

		// redirect to SSL if spec allows it, and given the system is configured to not allow HTTP in allowed-protocols
		if spec.AllowSSLRedirect && len(spec.AllowedProtocols) == 1 && spec.AllowedProtocols[0] == "https" {
			ingressAnnotations["nginx.ingress.kubernetes.io/ssl-redirect"] = AnnotationValue{Value: "true"}
		} else {
			ingressAnnotations["nginx.ingress.kubernetes.io/ssl-redirect"] = AnnotationValue{Value: "false"}
		}
	}

	if spec.ProxyReadTimeout != "" {
		ingressAnnotations["nginx.ingress.kubernetes.io/proxy-read-timeout"] = AnnotationValue{Value: spec.ProxyReadTimeout}
	}

	if spec.WhitelistIPAddresses != nil {
		ingressAnnotations["nginx.ingress.kubernetes.io/whitelist-source-range"] = AnnotationValue{Value: strings.Join(spec.WhitelistIPAddresses, ",")}
	}

	if spec.Annotations != nil {
		for annotation, annotationValue := range spec.Annotations {
			ingressAnnotations[annotation] = annotationValue
		}
	}

	quotedAnnotations := make(map[string]string)
	for annotation, annotationValue := range ingressAnnotations {
		if annotationValue.QuoteEscapingNeeded {
			quotedAnnotations[annotation] = common.Quote(annotationValue.Value)
		} else {
			quotedAnnotations[annotation] = fmt.Sprintf("\"%s\"", annotationValue.Value)
		}
	}

	templateText := `
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  annotations:
    {{- range $annotationKey, $annotationValue := .Annotations }}
    {{ $annotationKey }}: {{ $annotationValue }}
    {{- end }}
spec:
  tls:
  - hosts: [{{ .Host }}]
    {{ .TLSSecret }}
  rules:
  - host: {{ .Host }}
    http:
      paths:
      - path: {{ .Path }}
        backend:
          serviceName: {{ .ServiceName }}
          servicePort: {{ .ServicePort }}
{{- if .SecretResourceContents }}
---
{{ .SecretResourceContents }}
{{- end }}
`

	return common.RenderTemplate(templateText, map[string]interface{}{
		"Name":                   spec.Name,
		"Namespace":              spec.Namespace,
		"Annotations":            quotedAnnotations,
		"Host":                   spec.Host,
		"ServiceName":            spec.ServiceName,
		"ServicePort":            spec.ServicePort,
		"SecretResourceContents": secretResourceContents,
		"TLSSecret":              spec.TLSSecret,
		"Path":                   spec.Path,
	})
}

func getDexAuthIngressAnnotations(spec Spec) (map[string]AnnotationValue, error) {

	if spec.Authentication == nil || spec.Authentication.DexAuth == nil {
		return nil, errors.New("Dex auth spec is missing")
	}

	authURL := fmt.Sprintf("%s/oauth2/auth", spec.Authentication.DexAuth.Oauth2ProxyURL)
	signinURL := fmt.Sprintf("%s/oauth2/start?rd=https://$host$escaped_request_uri", spec.Authentication.DexAuth.Oauth2ProxyURL)

	return map[string]AnnotationValue{
		"nginx.ingress.kubernetes.io/auth-response-headers": {Value: "Authorization"},
		"nginx.ingress.kubernetes.io/auth-url":              {Value: authURL},
		"nginx.ingress.kubernetes.io/auth-signin":           {Value: signinURL},
		"nginx.ingress.kubernetes.io/configuration-snippet": {Value: `auth_request_set $name_upstream_1 $upstream_cookie__oauth2_proxy_1;
      
      access_by_lua_block {
        if ngx.var.name_upstream_1 ~= "" then
          ngx.header["Set-Cookie"] = "_oauth2_proxy_1=" .. ngx.var.name_upstream_1 .. ngx.var.auth_cookie:match("(; .*)")
        end
      }`, QuoteEscapingNeeded: true},
	}, nil
}

func getSessionVerificationAnnotations(sessionVerificationEndpoint,
	iguazioAuthURL,
	iguazioSigninURL string) (map[string]AnnotationValue, error) {

	return map[string]AnnotationValue{
		"nginx.ingress.kubernetes.io/auth-method":           {Value: "POST"},
		"nginx.ingress.kubernetes.io/auth-response-headers": {Value: "X-Remote-User,X-V3io-Session-Key"},
		"nginx.ingress.kubernetes.io/auth-url": {Value: fmt.Sprintf(
			"https://%s%s",
			iguazioAuthURL,
			sessionVerificationEndpoint)},
		"nginx.ingress.kubernetes.io/auth-signin": {Value: fmt.Sprintf(
			"https://%s/login", iguazioSigninURL)},
		"nginx.ingress.kubernetes.io/configuration-snippet": {
			Value:               "proxy_set_header authorization \"\";",
			QuoteEscapingNeeded: true,
		},
	}, nil
}

func getBasicAuthIngressAnnotationsAndSecret(ctx context.Context,
	spec Spec) (map[string]AnnotationValue, string, error) {

	if spec.Authentication == nil || spec.Authentication.BasicAuth == nil {
		return nil, "", errors.New("Basic auth spec is missing")
	}

	authSecretName := fmt.Sprintf("%s-basic-auth", spec.Authentication.BasicAuth.Name)

	// TODO: implement without htpasswd
	htpasswdContents := ""
	//htpasswdContents, err := common.GetHtpasswdContents(ctx,
	//	spec.Authentication.BasicAuth.Username,
	//	spec.Authentication.BasicAuth.Password,
	//	cmdRunnerSession)
	//if err != nil {
	//	return nil, "", errors.Wrap(err, "Failed to get htpasswd")
	//}

	ingressAnnotations := map[string]AnnotationValue{
		"nginx.ingress.kubernetes.io/auth-type":   {Value: "basic"},
		"nginx.ingress.kubernetes.io/auth-secret": {Value: authSecretName},
		"nginx.ingress.kubernetes.io/auth-realm":  {Value: "Authentication Required"},
	}

	templateText := `
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Name }}
type: Opaque
data:
  auth: {{ .EncodedHtpasswd }}
`

	renderedTemplate, err := common.RenderTemplate(templateText, map[string]interface{}{
		"Name":            authSecretName,
		"EncodedHtpasswd": base64.StdEncoding.EncodeToString([]byte(htpasswdContents)),
	})

	return ingressAnnotations, renderedTemplate, err
}
