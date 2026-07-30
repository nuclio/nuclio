/*
Copyright 2026 The Nuclio Authors.

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

package registryhelpers

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	_ "embed"
)

const (
	authSourcesVolumeName = "authsrc"
	authSourcesMountPath  = "/tmp/registry-auth-sources"

	authScriptVolumeName = "authscript"
	authScriptMountPath  = "/tmp/registry-auth-script"
	authScriptFileName   = "merge_authfile.py"

	MergeScriptConfigMapName = "nuclio-registry-auth-merge-script"
)

//go:embed merge_authfile.py
var mergeAuthFileScript string

// MergeScriptContents returns the embedded merge_authfile.py script, for use as ConfigMap data.
func MergeScriptContents() string {
	return mergeAuthFileScript
}

// BuildMergeAuthInitContainer returns the merge-authfile init container and its volumes; it runs the
// embedded Python merge script, delivered via the MergeScriptConfigMapName ConfigMap.
func BuildMergeAuthInitContainer(secretNames []string,
	authFileDir string,
	withCloudTokens bool,
	cfg AuthConfig) (v1.Container, []v1.Volume) {

	sources := make([]v1.VolumeProjection, 0, len(secretNames))
	for i, secretName := range secretNames {
		optional := true
		sources = append(sources, v1.VolumeProjection{
			Secret: &v1.SecretProjection{
				LocalObjectReference: v1.LocalObjectReference{Name: secretName},
				Items: []v1.KeyToPath{
					{Key: ".dockerconfigjson", Path: fmt.Sprintf("%d.json", i)},
				},
				Optional: &optional,
			},
		})
	}

	volumes := []v1.Volume{
		{
			Name:         authSourcesVolumeName,
			VolumeSource: v1.VolumeSource{Projected: &v1.ProjectedVolumeSource{Sources: sources}},
		},
		{
			Name: authScriptVolumeName,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{Name: MergeScriptConfigMapName},
				},
			},
		},
	}

	args := []string{
		fmt.Sprintf("--secrets=%s", authSourcesMountPath),
		fmt.Sprintf("--target=%s/config.json", authFileDir),
	}
	volumeMounts := []v1.VolumeMount{
		{Name: AuthVolumeName, MountPath: authFileDir},
		{Name: authSourcesVolumeName, MountPath: authSourcesMountPath, ReadOnly: true},
		{Name: authScriptVolumeName, MountPath: authScriptMountPath, ReadOnly: true},
	}
	if withCloudTokens {
		tokenMount := TokenDirVolumeMount()
		args = append(args, fmt.Sprintf("--cloud-tokens=%s", tokenMount.MountPath))
		volumeMounts = append(volumeMounts, tokenMount)
	}

	container := v1.Container{
		Name:            "merge-authfile",
		Image:           cfg.PythonImage,
		ImagePullPolicy: v1.PullPolicy(cfg.PythonImagePullPolicy),
		Command:         []string{"python3", fmt.Sprintf("%s/%s", authScriptMountPath, authScriptFileName)},
		Args:            args,
		VolumeMounts:    volumeMounts,
	}

	return container, volumes
}
