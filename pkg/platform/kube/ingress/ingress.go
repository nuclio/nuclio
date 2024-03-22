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

package ingress

import (
	"context"
	"fmt"
	"strings"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Resources keep resources needed for ingress creation
// (BasicAuthSecretName is used when it is an ingress with basic-auth authentication)
type Resources struct {
	Ingress *networkingv1.Ingress
}

type Manager struct {
	logger                logger.Logger
	cmdRunner             cmdrunner.CmdRunner
	kubeClientSet         kubernetes.Interface
	platformConfiguration *platformconfig.Config
}

func NewManager(parentLogger logger.Logger,
	kubecClientSet kubernetes.Interface,
	cmdRunner cmdrunner.CmdRunner,
	platformConfiguration *platformconfig.Config) (*Manager, error) {
	return &Manager{
		logger:                parentLogger.GetChild("manager"),
		cmdRunner:             cmdRunner,
		kubeClientSet:         kubecClientSet,
		platformConfiguration: platformConfiguration,
	}, nil
}

func (m *Manager) GenerateResources(ctx context.Context,
	spec Spec) (*Resources, error) {

	var err error

	ingressAnnotations, err := m.compileAnnotations(ctx, spec)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to compile ingress annotations")
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        spec.Name,
			Namespace:   spec.Namespace,
			Annotations: ingressAnnotations,
			Labels:      spec.Labels,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: spec.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: spec.Path,

									// TODO: make PathType configurable
									PathType: spec.PathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: spec.ServiceName,
											Port: networkingv1.ServiceBackendPort{
												Number: int32(spec.ServicePort),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	m.enrichLabels(spec, ingress.Labels)

	// if no specific TLS secret was given - set it to be system's TLS secret
	tlsSecret := spec.TLSSecret
	if tlsSecret == "" {
		tlsSecret = m.platformConfiguration.IngressConfig.TLSSecret
	}

	// if there's a TLS secret - populate the TLS spec
	if tlsSecret != "" {
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{spec.Host},
				SecretName: tlsSecret,
			},
		}
	}

	return &Resources{
		Ingress: ingress,
	}, nil
}

func (m *Manager) GenerateHtpasswdContents(ctx context.Context,
	username string,
	password string) ([]byte, error) {

	runResult, err := m.cmdRunner.Run(nil,
		fmt.Sprintf("echo %s | htpasswd -n -i %s", common.Quote(password), common.Quote(username)))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to run htpasswd command")
	}

	return []byte(runResult.Output), nil
}

func (m *Manager) CreateOrUpdateResources(ctx context.Context, resources *Resources) (*networkingv1.Ingress, error) {
	var appliedIngress *networkingv1.Ingress
	var err error

	m.logger.InfoWithCtx(ctx, "Creating/Updating ingress resources", "ingressName", resources.Ingress.Name)

	if appliedIngress, err = m.kubeClientSet.
		NetworkingV1().
		Ingresses(resources.Ingress.Namespace).
		Create(ctx, resources.Ingress, metav1.CreateOptions{}); err != nil {

		if !apierrors.IsAlreadyExists(err) {
			return nil, errors.Wrap(err, "Failed to create ingress")
		}

		// if the ingress already exists - update it
		m.logger.InfoWithCtx(ctx, "Ingress already exists. Updating it",
			"ingressName", resources.Ingress.Name)
		if appliedIngress, err = m.kubeClientSet.
			NetworkingV1().
			Ingresses(resources.Ingress.Namespace).
			Update(ctx, resources.Ingress, metav1.UpdateOptions{}); err != nil {

			return nil, errors.Wrap(err, "Failed to update ingress")
		}
		m.logger.InfoWithCtx(ctx, "Successfully updated ingress", "ingressName", resources.Ingress.Name)

	} else {
		m.logger.InfoWithCtx(ctx, "Successfully created ingress", "ingressName", resources.Ingress.Name)
	}

	return appliedIngress, nil
}

// DeleteByName deletes an ingress resource by name
// when deleteAuthSecret == true, delete related secret resource too
func (m *Manager) DeleteByName(ctx context.Context, ingressName string, namespace string, deleteAuthSecret bool) error {
	var ingress *networkingv1.Ingress
	var err error

	m.logger.InfoWithCtx(ctx, "Deleting ingress by name",
		"ingressName", ingressName,
		"deleteAuthSecret", deleteAuthSecret)

	// if deleteAuthSecret == true, fetch the secret name used by the ingress and delete it
	if deleteAuthSecret {

		// get the ingress object so we can find the secret name
		if ingress, err = m.kubeClientSet.
			NetworkingV1().
			Ingresses(namespace).
			Get(ctx, ingressName, metav1.GetOptions{}); err != nil {

			if apierrors.IsNotFound(err) {
				m.logger.DebugWithCtx(ctx, "Ingress resource not found. Aborting deletion",
					"ingressName", ingressName)
				return nil
			}

			return errors.Wrap(err, "Failed to get ingress resource on ingress deletion by name")
		}

		// if it has an auth secret - delete it
		secretName := ingress.Annotations["nginx.ingress.kubernetes.io/auth-secret"]
		if secretName != "" {

			m.logger.InfoWithCtx(ctx, "Deleting ingress's auth secret",
				"ingressName", ingressName,
				"secretName", secretName)

			if err = m.kubeClientSet.
				CoreV1().
				Secrets(namespace).
				Delete(ctx, secretName, metav1.DeleteOptions{}); err != nil {

				if apierrors.IsNotFound(err) {
					m.logger.DebugWithCtx(ctx, "Ingress's secret not found. Continuing with ingress deletion",
						"ingressName", ingressName,
						"secretName", secretName)

				} else {
					return errors.Wrap(err, "Failed to delete auth secret resource on ingress deletion")
				}

			} else {
				m.logger.DebugWithCtx(ctx, "Successfully deleted ingress's secret",
					"ingressName", ingressName,
					"secretName", secretName)
			}
		}
	}

	// delete the ingress resource
	if err = m.kubeClientSet.
		NetworkingV1().
		Ingresses(ingress.Namespace).
		Delete(ctx, ingressName, metav1.DeleteOptions{}); err != nil {

		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete ingress")
		}

		m.logger.DebugWithCtx(ctx, "Ingress resource was not found. Nothing to delete", "ingressName", ingressName)

		return nil
	}

	m.logger.DebugWithCtx(ctx, "Successfully deleted ingress", "ingressName", ingressName)

	return nil
}

func (m *Manager) compileAnnotations(ctx context.Context, spec Spec) (map[string]string, error) {
	var err error

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

		authIngressAnnotations, err = m.compileAuthAnnotations(ctx, spec)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to compile auth annotations")
		}

		// merge with existing annotation map
		for annotation, annotationValue := range authIngressAnnotations {
			ingressAnnotations[annotation] = annotationValue
		}

		ingressAnnotations["nginx.ingress.kubernetes.io/proxy-body-size"] = "0"

		// redirect to SSL if spec specifically required it, otherwise default to platformConfig's default value
		enableSSLRedirect := m.platformConfiguration.IngressConfig.EnableSSLRedirect
		if spec.EnableSSLRedirect != nil {
			enableSSLRedirect = *spec.EnableSSLRedirect
		}

		if enableSSLRedirect {
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

	return ingressAnnotations, nil
}

func (m *Manager) compileAuthAnnotations(ctx context.Context, spec Spec) (map[string]string, error) {
	var authIngressAnnotations map[string]string
	var err error

	switch spec.AuthenticationMode {
	case AuthenticationModeNone:
		// do nothing
	case AuthenticationModeBasicAuth:
		authIngressAnnotations, err = m.compileBasicAuthAnnotationsAndSecretName(ctx, spec)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get basic auth annotations")
		}
	case AuthenticationModeAccessKey:

		// relevant when running on iguazio platform
		authIngressAnnotations, err = m.compileIguazioSessionVerificationAnnotations()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get access key auth mode annotations")
		}
	case AuthenticationModeOauth2:
		authIngressAnnotations, err = m.compileDexAuthAnnotations(spec)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get dex auth annotations")
		}
	default:
		return nil, errors.Errorf("Unknown ingress authentication mode: %s", spec.AuthenticationMode)
	}

	return authIngressAnnotations, nil
}

func (m *Manager) compileDexAuthAnnotations(spec Spec) (map[string]string, error) {

	oauth2ProxyURL := m.platformConfiguration.IngressConfig.Oauth2ProxyURL
	if spec.Authentication != nil && spec.Authentication.DexAuth != nil && spec.Authentication.DexAuth.Oauth2ProxyURL != "" {
		oauth2ProxyURL = spec.Authentication.DexAuth.Oauth2ProxyURL
	}

	addSignInAnnotation := false
	if spec.Authentication != nil && spec.Authentication.DexAuth != nil && spec.Authentication.DexAuth.RedirectUnauthorizedToSignIn {
		addSignInAnnotation = true
	}

	if oauth2ProxyURL == "" {
		return nil, errors.New("Oauth2 proxy URL is missing")
	}

	authURL := fmt.Sprintf("%s/oauth2/auth", oauth2ProxyURL)

	annotations := map[string]string{
		"nginx.ingress.kubernetes.io/auth-response-headers": "Authorization",
		"nginx.ingress.kubernetes.io/auth-url":              authURL,
		"nginx.ingress.kubernetes.io/configuration-snippet": `auth_request_set $name_upstream_1 $upstream_cookie__oauth2_proxy_1;
access_by_lua_block {
  if ngx.var.name_upstream_1 ~= "" then
    ngx.header["Set-Cookie"] = "_oauth2_proxy_1=" .. ngx.var.name_upstream_1 .. ngx.var.auth_cookie:match("(; .*)")
  end
}`,
	}

	if addSignInAnnotation {
		signinURL := fmt.Sprintf("%s/oauth2/start?rd=https://$host$escaped_request_uri", oauth2ProxyURL)
		annotations["nginx.ingress.kubernetes.io/auth-signin"] = signinURL
	}

	return annotations, nil
}

func (m *Manager) compileIguazioSessionVerificationAnnotations() (map[string]string, error) {
	if m.platformConfiguration.IngressConfig.IguazioAuthURL == "" {
		return nil, errors.New("No iguazio auth URL configured")
	}

	if m.platformConfiguration.IngressConfig.IguazioSignInURL == "" {
		return nil, errors.New("No iguazio sign in URL configured")
	}

	return map[string]string{
		"nginx.ingress.kubernetes.io/auth-method":           "POST",
		"nginx.ingress.kubernetes.io/auth-response-headers": "X-Remote-User,X-V3io-Session-Key",
		"nginx.ingress.kubernetes.io/auth-url":              m.platformConfiguration.IngressConfig.IguazioAuthURL,
		"nginx.ingress.kubernetes.io/configuration-snippet": "proxy_set_header authorization \"\";",
	}, nil
}

func (m *Manager) compileBasicAuthAnnotationsAndSecretName(ctx context.Context, spec Spec) (map[string]string, error) {

	if spec.Authentication == nil || spec.Authentication.BasicAuth == nil {
		return nil, errors.New("Basic auth spec is missing")
	}

	authSecretName := fmt.Sprintf("%s-basic-auth", spec.Authentication.BasicAuth.Name)

	ingressAnnotations := map[string]string{
		"nginx.ingress.kubernetes.io/auth-type":   "basic",
		"nginx.ingress.kubernetes.io/auth-secret": authSecretName,
		"nginx.ingress.kubernetes.io/auth-realm":  "Authentication Required",
	}
	return ingressAnnotations, nil
}

func (m *Manager) enrichLabels(spec Spec, labels map[string]string) {
	labels[common.NuclioLabelKeyClass] = "apigateway"
	labels[common.NuclioLabelKeyApp] = "ingress-manager"
	labels[common.NuclioResourceLabelKeyApiGatewayName] = spec.APIGatewayName
	labels[common.NuclioResourceLabelKeyProjectName] = spec.ProjectName
}
