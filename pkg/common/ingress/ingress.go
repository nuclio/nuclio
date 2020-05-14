package ingress

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio/pkg/common"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strings"
)

type SystemSpec struct {
	IngressTLSSecret string
	IguazioSigninURL string
	IguazioAuthURL string
}

func GenerateIngressResource(ctx context.Context,
	spec Spec) (*v1beta1.Ingress, *v1.Secret,  error) {

	var err error
	var secretResource *v1.Secret

	ingressAnnotations := map[string]string{
		"kubernetes.io/ingress.class": "nginx",
	}
	if spec.RewriteTarget != "" {
		ingressAnnotations["nginx.ingress.kubernetes.io/rewrite-target"] = spec.RewriteTarget
	}

	if spec.UpstreamVhost != "" {
		ingressAnnotations["nginx.ingress.kubernetes.io/upstream-vhost"] = spec.UpstreamVhost
	}

	if spec.BackendProtocol != "" {
		ingressAnnotations["nginx.ingress.kubernetes.io/backend-protocol"] = spec.BackendProtocol
	}

	if spec.SSLPassthrough {
		ingressAnnotations["nginx.ingress.kubernetes.io/ssl-passthrough"] = "true"
	} else {
		var authIngressAnnotations map[string]string

		switch spec.AuthenticationMode {
		case AuthenticationModeNone:
			//do nothing
		case AuthenticationModeBasicAuth:
			authIngressAnnotations, secretResource, err = getBasicAuthIngressAnnotationsAndSecret(ctx, spec)
			if err != nil {
				return nil, nil, errors.Wrap(err, "Failed to get basic auth annotations")
			}
		case AuthenticationModeAccessKey:
			authIngressAnnotations, err = getSessionVerificationAnnotations("/api/data_sessions/verifications")
			if err != nil {
				return nil, nil, errors.Wrap(err, "Failed to get access key auth mode annotations")
			}
		case AuthenticationModeDex:
			authIngressAnnotations, err = getDexAuthIngressAnnotations(spec)
			if err != nil {
				return nil, nil, errors.Wrap(err, "Failed to get dex auth annotations")
			}
		default:
			return nil, nil, errors.Errorf("Unknown ingress authentication mode: %s", spec.AuthenticationMode)
		}

		// merge with existing annotation map
		for annotation, annotationValue := range authIngressAnnotations {
			ingressAnnotations[annotation] = annotationValue
		}

		ingressAnnotations["nginx.ingress.kubernetes.io/proxy-body-size"] = "0"

		// redirect to SSL if spec allows it, and given the system is configured to not allow HTTP in allowed-protocols
		if spec.AllowSSLRedirect && len(spec.AllowedProtocols) == 1 && spec.AllowedProtocols[0] == "https" {
			ingressAnnotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
		} else {
			ingressAnnotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "false"
		}
	}

	if spec.ProxyReadTimeout != "" {
		ingressAnnotations["nginx.ingress.kubernetes.io/proxy-read-timeout"] = spec.ProxyReadTimeout
	}

	if spec.WhitelistIPAddresses != nil {
		ingressAnnotations["nginx.ingress.kubernetes.io/whitelist-source-range"] = strings.Join(spec.WhitelistIPAddresses, ",")
	}

	if spec.Annotations != nil {
		for annotation, annotationValue := range spec.Annotations {
			ingressAnnotations[annotation] = annotationValue
		}
	}

	// if no specific TLS secret was given - set it to be system's TLS secret
	tlsSecret := spec.TLSSecret
	if tlsSecret == "" {
		tlsSecret = common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_INGRESS_TLS_SECRET", "")
	}

	ingressResource := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: spec.Name,
			Namespace: spec.Namespace,
			Annotations: ingressAnnotations,
		},
		Spec: v1beta1.IngressSpec{
			TLS: []v1beta1.IngressTLS{
				{
					Hosts: []string{spec.Host},
					SecretName: tlsSecret,
				},
			},
			Rules: []v1beta1.IngressRule{
				{
					Host: spec.Host,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: spec.Path,
									Backend: v1beta1.IngressBackend{
										ServiceName: spec.ServiceName,
										ServicePort: intstr.FromInt(spec.ServicePort),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return ingressResource, secretResource, nil
}

func getDexAuthIngressAnnotations(spec Spec) (map[string]string, error) {

	if spec.Authentication == nil || spec.Authentication.DexAuth == nil {
		return nil, errors.New("Dex auth spec is missing")
	}

	authURL := fmt.Sprintf("%s/oauth2/auth", spec.Authentication.DexAuth.Oauth2ProxyURL)
	signinURL := fmt.Sprintf("%s/oauth2/start?rd=https://$host$escaped_request_uri", spec.Authentication.DexAuth.Oauth2ProxyURL)

	return map[string]string{
		"nginx.ingress.kubernetes.io/auth-response-headers": "Authorization",
		"nginx.ingress.kubernetes.io/auth-url":              authURL,
		"nginx.ingress.kubernetes.io/auth-signin":           signinURL,
		"nginx.ingress.kubernetes.io/configuration-snippet": `auth_request_set $name_upstream_1 $upstream_cookie__oauth2_proxy_1;
      
      access_by_lua_block {
        if ngx.var.name_upstream_1 ~= "" then
          ngx.header["Set-Cookie"] = "_oauth2_proxy_1=" .. ngx.var.name_upstream_1 .. ngx.var.auth_cookie:match("(; .*)")
        end
      }`,
	}, nil
}

func getSessionVerificationAnnotations(sessionVerificationEndpoint string) (map[string]string, error) {

	return map[string]string{
		"nginx.ingress.kubernetes.io/auth-method": "POST",
		"nginx.ingress.kubernetes.io/auth-response-headers": "X-Remote-User,X-V3io-Session-Key",
		"nginx.ingress.kubernetes.io/auth-url": fmt.Sprintf(
			"https://%s%s",
			common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_IGUAZIO_AUTH_URL", ""),
			sessionVerificationEndpoint),
		"nginx.ingress.kubernetes.io/auth-signin": fmt.Sprintf("https://%s/login",
			common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_IGUAZIO_SIGNIN_URL", "")),
		"nginx.ingress.kubernetes.io/configuration-snippet": "proxy_set_header authorization \"\";",
	}, nil
}

func getBasicAuthIngressAnnotationsAndSecret(ctx context.Context,
	spec Spec) (map[string]string, *v1.Secret, error) {
	var encodedHtpasswdContents []byte

	if spec.Authentication == nil || spec.Authentication.BasicAuth == nil {
		return nil, nil, errors.New("Basic auth spec is missing")
	}

	authSecretName := fmt.Sprintf("%s-basic-auth", spec.Authentication.BasicAuth.Name)

	htpasswdContents, err := GenerateHtpasswdContents(ctx,
		spec.Authentication.BasicAuth.Username,
		spec.Authentication.BasicAuth.Password)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to generate htpasswd contents")
	}

	ingressAnnotations := map[string]string{
		"nginx.ingress.kubernetes.io/auth-type":   "basic",
		"nginx.ingress.kubernetes.io/auth-secret": authSecretName,
		"nginx.ingress.kubernetes.io/auth-realm":  "Authentication Required",
	}

	// encode secret auth
	base64.StdEncoding.Encode(encodedHtpasswdContents, []byte(htpasswdContents))

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: authSecretName,
			Namespace: spec.Namespace,
		},
		Type: v1.SecretType("Opaque"),
		Data: map[string][]byte {
			"auth": encodedHtpasswdContents,
		},
	}

	return ingressAnnotations, secret, nil
}

func GenerateHtpasswdContents(ctx context.Context,
	username string,
	password string) ([]byte, error) {

	cmd := exec.CommandContext(ctx,
		"htpasswdGeneration",
		fmt.Sprintf("echo %s | htpasswd -n -i %s", common.Quote(password), username))

	return cmd.Output()
}
