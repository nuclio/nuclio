//go:build test_unit

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

package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type k8sTestSuite struct {
	suite.Suite
	ctx           context.Context
	kubeClientSet *fake.Clientset
}

func (suite *k8sTestSuite) SetupSuite() {
	suite.kubeClientSet = fake.NewSimpleClientset()
	suite.ctx = context.Background()
}

func (suite *k8sTestSuite) TestFilterInvalidLabels() {
	invalidLabels := map[string]string{
		"my@weird/label":   "value",
		"my.wierd/label":   "value@",
		"%weird+/label":    "v8$alue",
		"email.like@label": "value",
	}

	labels := map[string]string{
		"valid":          "label",
		"another-valid":  "label-value",
		"also_123_valid": "label_456_value",
	}

	// add invalid labels to labels
	for key, value := range invalidLabels {
		labels[key] = value
	}

	filteredLabels := FilterInvalidLabels(labels)

	suite.Require().Equal(len(filteredLabels), len(labels)-len(invalidLabels))
	for key := range invalidLabels {
		_, ok := filteredLabels[key]
		suite.Require().False(ok, "invalid label %s should not be in filtered labels", key)
	}
}

func (suite *k8sTestSuite) TestEnrichDefaultReadinessProbe() {
	testNum14 := int32(14)
	testNum17 := int32(17)
	for _, testCase := range []struct {
		name           string
		probeConfig    *v1.Probe
		defaultProbe   *v1.Probe
		expectedResult *v1.Probe
	}{
		{
			name:           "enrich all ReadinessProbe in nil case",
			probeConfig:    nil,
			defaultProbe:   suite.newTestProbe(testNum17),
			expectedResult: suite.newTestProbe(testNum17),
		}, {
			name: "enrich defaults besides TimeoutSeconds",
			probeConfig: &v1.Probe{
				TimeoutSeconds: testNum14,
			},
			defaultProbe: suite.newTestProbe(testNum17),
			expectedResult: &v1.Probe{
				InitialDelaySeconds: testNum17,
				TimeoutSeconds:      testNum14,
				PeriodSeconds:       testNum17,
				FailureThreshold:    testNum17,
			},
		}, {
			name:           "don't enrich anything",
			probeConfig:    suite.newTestProbe(testNum17),
			defaultProbe:   suite.newTestProbe(testNum14),
			expectedResult: suite.newTestProbe(testNum17),
		},
	} {
		suite.Run(testCase.name, func() {
			EnrichProbe(&testCase.probeConfig, testCase.defaultProbe)
			suite.Require().NotEmpty(testCase.probeConfig)
			suite.Require().Equal(testCase.expectedResult, testCase.probeConfig)
		})
	}
}

func (suite *k8sTestSuite) TestEnrichDefaultLivenessProbe() {
	testNum14 := int32(14)
	testNum17 := int32(17)
	for _, testCase := range []struct {
		name           string
		probeConfig    *v1.Probe
		defaultProbe   *v1.Probe
		expectedResult *v1.Probe
	}{
		{
			name:           "enrich all LivenessProbe in nil case",
			probeConfig:    nil,
			defaultProbe:   suite.newTestProbe(testNum17),
			expectedResult: suite.newTestProbe(testNum17),
		}, {
			name: "enrich defaults besides TimeoutSeconds",
			probeConfig: &v1.Probe{
				TimeoutSeconds: testNum14,
			},
			defaultProbe: suite.newTestProbe(testNum17),
			expectedResult: &v1.Probe{
				InitialDelaySeconds: testNum17,
				TimeoutSeconds:      testNum14,
				PeriodSeconds:       testNum17,
				FailureThreshold:    testNum17,
			},
		}, {
			name:           "don't enrich anything",
			probeConfig:    suite.newTestProbe(testNum17),
			defaultProbe:   suite.newTestProbe(testNum14),
			expectedResult: suite.newTestProbe(testNum17),
		},
	} {
		suite.Run(testCase.name, func() {
			EnrichProbe(&testCase.probeConfig, testCase.defaultProbe)
			suite.Require().NotEmpty(testCase.probeConfig)
			suite.Require().Equal(testCase.expectedResult, testCase.probeConfig)
		})
	}
}

func (suite *k8sTestSuite) newTestProbe(value int32) *v1.Probe {
	return &v1.Probe{
		InitialDelaySeconds: value,
		TimeoutSeconds:      value,
		PeriodSeconds:       value,
		FailureThreshold:    value,
	}
}

func (suite *k8sTestSuite) TestRenderProjectSecretName() {
	templateData := map[string]interface{}{
		"ProjectName": "myproj",
		"Namespace":   "ns1",
	}
	testCases := []struct {
		name               string
		template           string
		expectedSecretName string
	}{
		{
			name:               "WithProjectAndNamespace",
			template:           "nuclio-project-secrets-{{ .ProjectName }}-{{ .Namespace }}",
			expectedSecretName: "nuclio-project-secrets-myproj-ns1",
		},
		{
			name:               "WithProjectOnly",
			template:           "nuclio-project-secrets-{{ .ProjectName }}",
			expectedSecretName: "nuclio-project-secrets-myproj",
		},
		{
			name:               "Empty",
			expectedSecretName: "",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {

			secretName, err := renderProjectSecretName(testCase.template, templateData)
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.expectedSecretName, secretName)
		})
	}
}

func (suite *k8sTestSuite) TestGetProjectSecret() {
	kubeClient := fake.NewSimpleClientset()
	namespace := "ns1"
	projectName := "myproj"
	template := "nuclio-project-secrets-{{ .ProjectName }}-{{ .Namespace }}"
	secretName, _ := renderProjectSecretName(template, map[string]interface{}{
		"ProjectName": projectName,
		"Namespace":   namespace,
	})

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"key": []byte("value"),
		},
	}
	_, err := kubeClient.CoreV1().Secrets(namespace).Create(suite.ctx, secret, metav1.CreateOptions{})
	suite.Require().NoError(err)

	testCases := []struct {
		name                string
		template            string
		projectName         string
		namespace           string
		expectedSecretFound bool
	}{
		{
			name:                "SecretFound",
			template:            template,
			projectName:         projectName,
			namespace:           namespace,
			expectedSecretFound: true,
		},
		{
			name:                "TemplateEmpty",
			template:            "",
			projectName:         projectName,
			namespace:           namespace,
			expectedSecretFound: false,
		},
		{
			name:                "SecretNotFound",
			template:            template,
			projectName:         "otherproj",
			namespace:           namespace,
			expectedSecretFound: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			foundSecret, err := GetProjectSecret(suite.ctx, kubeClient, tc.template, tc.projectName, tc.namespace)
			suite.Require().NoError(err)
			if tc.expectedSecretFound {
				suite.Require().NotNil(foundSecret)
				suite.Require().Equal(secretName, foundSecret.Name)
			} else {
				suite.Require().Nil(foundSecret)
			}
		})
	}
}

func (suite *k8sTestSuite) TestIsServiceAccountAllowed() {
	secretDataMultiple := &v1.Secret{
		Data: map[string][]byte{
			"allowed": []byte("sa1,sa2"),
		},
	}
	secretDataSingle := &v1.Secret{
		Data: map[string][]byte{
			"allowed": []byte("sa1"),
		},
	}
	testCases := []struct {
		name      string
		secret    *v1.Secret
		key       string
		sa        string
		expectErr bool
	}{
		{
			name:      "AllowedFromMultiple",
			secret:    secretDataMultiple,
			key:       "allowed",
			sa:        "sa1",
			expectErr: false,
		},
		{
			name:      "AllowedFromSingle",
			secret:    secretDataSingle,
			key:       "allowed",
			sa:        "sa1",
			expectErr: false,
		},
		{
			name:      "NotAllowed",
			secret:    secretDataMultiple,
			key:       "allowed",
			sa:        "sa3",
			expectErr: true,
		},
		{
			name:      "NoRestrictionKeyMissing",
			secret:    secretDataMultiple,
			key:       "missing",
			sa:        "sa1",
			expectErr: false,
		},
		{
			name: "KeyFoundButEmpty",
			secret: &v1.Secret{
				Data: map[string][]byte{
					"allowed": []byte(""),
				},
			},
			key:       "allowed",
			sa:        "sa1",
			expectErr: true,
		},
		{
			name:      "AllowedCaseInsensitive",
			secret:    secretDataMultiple,
			key:       "allowed",
			sa:        "SA1",
			expectErr: false,
		},
		{
			name: "AllowedCaseInsensitiveMixed",
			secret: &v1.Secret{
				Data: map[string][]byte{
					"allowed": []byte("Sa1,SA2"),
				},
			},
			key:       "allowed",
			sa:        "sa2",
			expectErr: false,
		},
		{
			name:      "NotAllowedCaseInsensitive",
			secret:    secretDataMultiple,
			key:       "allowed",
			sa:        "Sa3",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			err := IsServiceAccountAllowed(tc.secret, tc.key, tc.sa)
			if tc.expectErr {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *k8sTestSuite) TestEnrichServiceAccount() {
	secretData := &v1.Secret{
		Data: map[string][]byte{
			"default": []byte("sa-default"),
		},
	}
	defaultPlatformServiceAccount := "sa-platform-default"
	testCases := []struct {
		name     string
		secret   *v1.Secret
		key      string
		sa       string
		expected string
	}{
		{
			name:     "ExplicitServiceAccount",
			secret:   secretData,
			key:      "default",
			sa:       "sa-explicit",
			expected: "sa-explicit",
		},
		{
			name:     "DefaultFromSecret",
			secret:   secretData,
			key:      "default",
			expected: "sa-default",
		},
		{
			name:     "KeyMissing",
			secret:   secretData,
			key:      "missing",
			expected: defaultPlatformServiceAccount,
		},
		{
			name:     "NilSecret",
			secret:   nil,
			key:      "default",
			expected: defaultPlatformServiceAccount,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := EnrichServiceAccount(tc.secret, tc.key, tc.sa, defaultPlatformServiceAccount)
			suite.Require().Equal(tc.expected, result)
		})
	}
}

func (suite *k8sTestSuite) TestGetAllowedServiceAccountsFromSecret() {
	secretData := &v1.Secret{
		Data: map[string][]byte{
			"allowed": []byte("sa1,sa2"),
		},
	}

	testCases := []struct {
		name     string
		secret   *v1.Secret
		key      string
		expected []string
		found    bool
	}{
		{
			name:     "KeyFound",
			secret:   secretData,
			key:      "allowed",
			expected: []string{"sa1", "sa2"},
			found:    true,
		},
		{
			name:   "KeyMissing",
			secret: secretData,
			key:    "missing",
		},
		{
			name: "NilSecret",
			key:  "allowed",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			allowed, found := getAllowedServiceAccountsFromSecret(tc.secret, tc.key)
			suite.Require().Equal(tc.found, found)
			suite.Require().Equal(tc.expected, allowed)
		})
	}
}

func (suite *k8sTestSuite) TestEnrichAndValidateServiceAccount() {
	namespace := "ns1"
	projectName := "myproj"
	template := "nuclio-project-secrets-{{ .ProjectName }}-{{ .Namespace }}"
	defaultKey := "default"
	allowedKey := "allowed"
	defaultPlatformServiceAccount := "sa-platform-default"

	secretName, _ := renderProjectSecretName(template, map[string]interface{}{
		"ProjectName": projectName,
		"Namespace":   namespace,
	})

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			defaultKey: []byte("sa-default"),
			allowedKey: []byte("sa1,sa2"),
		},
	}

	testCases := []struct {
		name           string
		serviceAccount string
		shouldEnrich   bool
		secretData     map[string][]byte
		expectedSA     string
		expectErr      bool
		template       string
		secretExists   bool
	}{
		{
			name:           "AllowedExplicitSA",
			serviceAccount: "sa1",
			secretData:     secret.Data,
			expectedSA:     "sa1",
			template:       template,
			secretExists:   true,
		},
		{
			name:           "AllowedEnrichedSA",
			serviceAccount: "",
			shouldEnrich:   true,
			secretData:     secret.Data,
			expectedSA:     "sa-default",
			expectErr:      true, // sa-default is not in allowed list
			template:       template,
			secretExists:   true,
		},
		{
			name:           "NotAllowedSA",
			serviceAccount: "sa3",
			secretData:     secret.Data,
			expectErr:      true,
			template:       template,
			secretExists:   true,
		},
		{
			name:           "NoRestrictionKeyMissing",
			serviceAccount: "sa-any",
			secretData: map[string][]byte{
				defaultKey: []byte("sa-default"),
			},
			expectedSA:   "sa-any",
			template:     template,
			secretExists: true,
		},
		{
			name:           "SecretNotFound",
			serviceAccount: "sa-any",
			expectedSA:     "sa-any",
			template:       template,
		},
		{
			name:           "RenderSecretNameError",
			serviceAccount: "sa-any",
			secretData:     secret.Data,
			expectErr:      true,
			template:       "{{ .ProjectName }", // invalid template
			secretExists:   true,
		},
		{
			name:           "DefaultPlatformServiceAccountUsed",
			serviceAccount: "",
			shouldEnrich:   true,
			secretData:     map[string][]byte{}, // no default key in secret
			expectedSA:     defaultPlatformServiceAccount,
			template:       template,
			secretExists:   true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			_ = suite.kubeClientSet.CoreV1().Secrets(namespace).Delete(suite.ctx, secretName, metav1.DeleteOptions{})

			if tc.secretExists && tc.secretData != nil {
				secret.Data = tc.secretData
				_, err := suite.kubeClientSet.CoreV1().Secrets(namespace).Create(suite.ctx, secret, metav1.CreateOptions{})
				suite.Require().NoError(err)
			}

			sa, err := EnrichAndValidateServiceAccount(
				suite.ctx,
				suite.kubeClientSet,
				defaultPlatformServiceAccount,
				tc.template,
				defaultKey,
				allowedKey,
				tc.serviceAccount,
				projectName,
				namespace,
				tc.shouldEnrich,
			)
			if tc.expectErr {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(tc.expectedSA, sa)
			}
		})
	}
}

func TestK8sTestSuite(t *testing.T) {
	suite.Run(t, new(k8sTestSuite))
}
