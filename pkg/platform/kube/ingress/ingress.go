package ingress

import (
	"context"
	"fmt"
	"strings"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// keeps resources needed for ingress creation
// (secret is used when it is an ingress with basic-auth authentication)
type Resources struct {
	Ingress *v1beta1.Ingress
	Secret *v1.Secret
}

type Manager struct {
	logger        logger.Logger
	cmdRunner     cmdrunner.CmdRunner
	kubeClientSet kubernetes.Interface
}

func NewManager (parentLogger logger.Logger,
	kubecClientSet kubernetes.Interface,
	cmdRunner cmdrunner.CmdRunner) (*Manager, error) {

	return &Manager{
		logger:        parentLogger.GetChild("runner"),
		cmdRunner:     cmdRunner,
		kubeClientSet: kubecClientSet,
	}, nil
}

func (im *Manager) GenerateIngressResources(ctx context.Context,
	spec Spec) (*Resources,  error) {

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
			authIngressAnnotations, secretResource, err = im.getBasicAuthIngressAnnotationsAndSecret(ctx, spec)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to get basic auth annotations")
			}
		case AuthenticationModeAccessKey:

			// relevant when running on iguazio platform
			authIngressAnnotations, err = im.getSessionVerificationAnnotations("/api/data_sessions/verifications")
			if err != nil {
				return nil, errors.Wrap(err, "Failed to get access key auth mode annotations")
			}
		case AuthenticationModeDex:
			authIngressAnnotations, err = im.getDexAuthIngressAnnotations(spec)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to get dex auth annotations")
			}
		default:
			return nil, errors.Errorf("Unknown ingress authentication mode: %s", spec.AuthenticationMode)
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

	ingressResource := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: spec.Name,
			Namespace: spec.Namespace,
			Annotations: ingressAnnotations,
		},
		Spec: v1beta1.IngressSpec{
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

	// if no specific TLS secret was given - set it to be system's TLS secret
	tlsSecret := spec.TLSSecret
	if tlsSecret == "" {
		tlsSecret = common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_INGRESS_TLS_SECRET", "")
	}

	// if there's a TLS secret - populate the TLS spec
	if tlsSecret != "" {
		ingressResource.Spec.TLS = []v1beta1.IngressTLS{
			{
				Hosts: []string{spec.Host},
				SecretName: tlsSecret,
			},
		}
	}

	return &Resources{
		Ingress: ingressResource,
		Secret: secretResource,
	}, nil
}

func (im *Manager) getDexAuthIngressAnnotations(spec Spec) (map[string]string, error) {

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

func (im *Manager) getSessionVerificationAnnotations(sessionVerificationEndpoint string) (map[string]string, error) {

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

func (im *Manager) getBasicAuthIngressAnnotationsAndSecret(ctx context.Context,
	spec Spec) (map[string]string, *v1.Secret, error) {

	if spec.Authentication == nil || spec.Authentication.BasicAuth == nil {
		return nil, nil, errors.New("Basic auth spec is missing")
	}

	// validate mandatory fields existence
	for fieldName, field := range map[string]string{
		"name":     spec.Authentication.BasicAuth.Name,
		"username": spec.Authentication.BasicAuth.Username,
		"password": spec.Authentication.BasicAuth.Password,
	} {
		if field == "" {
			return nil, nil, errors.Errorf("Missing mandatory field in spec: %s", fieldName)
		}
	}

	authSecretName := fmt.Sprintf("%s-basic-auth", spec.Authentication.BasicAuth.Name)

	htpasswdContents, err := im.GenerateHtpasswdContents(ctx,
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

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: authSecretName,
			Namespace: spec.Namespace,
		},
		Type: v1.SecretType("Opaque"),
		Data: map[string][]byte {
			"auth": htpasswdContents,
		},
	}

	return ingressAnnotations, secret, nil
}

func (im *Manager) GenerateHtpasswdContents(ctx context.Context,
	username string,
	password string) ([]byte, error) {

	runResult, err := im.cmdRunner.Run(nil,
		fmt.Sprintf("echo %s | htpasswd -n -i %s", common.Quote(password), common.Quote(username)))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to run htpasswd command")
	}

	return []byte(runResult.Output), nil
}

func (im *Manager) CreateOrUpdateIngressResources(ingressResources *Resources) (*v1beta1.Ingress, *v1.Secret, error) {
	var appliedIngress *v1beta1.Ingress
	var appliedSecret *v1.Secret
	var err error

	im.logger.InfoWith("Creating/Updating ingress resources",
		"ingressName", ingressResources.Ingress.Name)

	if appliedIngress, err = im.kubeClientSet.
		ExtensionsV1beta1().
		Ingresses(ingressResources.Ingress.Namespace).
		Create(ingressResources.Ingress); err != nil {

		// if the ingress already exists - update it
		if apierrors.IsAlreadyExists(err) {

			im.logger.InfoWith("Ingress already exists. Updating it",
				"ingressName", ingressResources.Ingress.Name)
			if appliedIngress, err = im.kubeClientSet.
				ExtensionsV1beta1().
				Ingresses(ingressResources.Ingress.Namespace).
				Update(ingressResources.Ingress); err != nil {

				return nil, nil, errors.Wrap(err, "Failed to update ingress")
			}
			im.logger.InfoWith("Successfully updated ingress",
				"ingressName", ingressResources.Ingress.Name)

		} else {

			return nil, nil, errors.Wrap(err, "Failed to create ingress")
		}
	} else {
		im.logger.InfoWith("Successfully created ingress",
			"ingressName", ingressResources.Ingress.Name)
	}

	// if there's a secret among the ingress resources - create/update it
	if ingressResources.Secret != nil {

		im.logger.InfoWith("Creating/Updating ingress's secret",
			"ingressName", ingressResources.Ingress.Name,
			"secretName", ingressResources.Secret.Name)
		if appliedSecret, err = im.kubeClientSet.
			CoreV1().
			Secrets(ingressResources.Secret.Namespace).
			Create(ingressResources.Secret); err != nil {

			// if the secret already exists - update it
			if apierrors.IsAlreadyExists(err) {

				im.logger.InfoWith("Secret already exists. Updating it",
					"secretName", ingressResources.Secret.Name)
				if appliedSecret, err = im.kubeClientSet.
					CoreV1().
					Secrets(ingressResources.Secret.Namespace).
					Update(ingressResources.Secret); err != nil {

					return nil, nil, errors.Wrap(err, "Failed to update secret")
				}
				im.logger.InfoWith("Successfully updated secret",
					"secretName", ingressResources.Secret.Name)

			} else {
				return nil, nil, errors.Wrap(err, "Failed to create secret")
			}
		} else {
			im.logger.InfoWith("Successfully created secret",
				"secretName", ingressResources.Secret.Name)
		}
	}

	return appliedIngress, appliedSecret, nil
}

// deletes ingress resource
// when deleteAuthSecret == true, delete related secret resource too
func (im *Manager) DeleteIngressByName(ingressName string, namespace string,  deleteAuthSecret bool) error {
	var ingress *v1beta1.Ingress
	var err error

	im.logger.InfoWith("Deleting ingress by name",
		"ingressName", ingressName,
	"deleteAuthSecret", deleteAuthSecret)

	// if deleteAuthSecret == true, fetch the secret name used by the ingress and delete it
	if deleteAuthSecret {

		// get the ingress object so we can find the secret name
		if ingress, err = im.kubeClientSet.
			ExtensionsV1beta1().
			Ingresses(namespace).
			Get(ingressName, metav1.GetOptions{}); err != nil {

			if !apierrors.IsNotFound(err) {
				return errors.Wrap(err, "Failed to get ingress resource on ingress deletion by name")
			}

			im.logger.DebugWith("Ingress resource not found. Aborting deletion",
				"ingressName", ingressName)
			return nil
		}

		// if it has an auth secret - delete it
		secretName := ingress.Annotations["nginx.ingress.kubernetes.io/auth-secret"]
		if secretName != "" {

			im.logger.InfoWith("Deleting ingress's auth secret",
				"ingressName", ingressName,
				"secretName", secretName)

			if err = im.kubeClientSet.
				CoreV1().
				Secrets(namespace).
				Delete(secretName, &metav1.DeleteOptions{}); err != nil {

				if !apierrors.IsNotFound(err) {
					return errors.Wrap(err, "Failed to delete auth secret resource on ingress deletion")
				}

				im.logger.DebugWith("Ingress's secret not found. Continuing with ingress deletion",
					"ingressName", ingressName,
					"secretName", secretName)

			} else {
				im.logger.DebugWith("Successfully deleted ingress's secret",
					"ingressName", ingressName,
					"secretName", secretName)
			}
		}
	}

	// delete the ingress resource
	if err = im.kubeClientSet.
		ExtensionsV1beta1().
		Ingresses(ingress.Namespace).
		Delete(ingressName, &metav1.DeleteOptions{}); err != nil {

		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "Failed to delete ingress")
		}

		im.logger.DebugWith("Ingress resource was not found. Nothing to delete",
			"ingressName", ingressName)

	} else {
		im.logger.DebugWith("Successfully deleted ingress",
			"ingressName", ingressName)
	}

	return nil
}
