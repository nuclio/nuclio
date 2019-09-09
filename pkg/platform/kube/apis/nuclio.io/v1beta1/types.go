package v1beta1

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuclioFunction describes a function.
type NuclioFunction struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   functionconfig.Spec   `json:"spec"`
	Status functionconfig.Status `json:"status,omitempty"`
}

func (nf *NuclioFunction) GetSpecReplicas() *int32 {
	var replicas int32

	// only when function is scaled to zero or disabled, allow for replicas to be set to zero
	if nf.Spec.Disabled || nf.Status.State == functionconfig.FunctionStateScaledToZero {
		replicas = 0
	} else if nf.Spec.Replicas != nil {
		if *nf.Spec.Replicas <= 0 {
			replicas = 0
		} else {
			replicas = *nf.Spec.Replicas
		}
	} else {
		minReplicas := nf.GetSpecMinReplicas()

		if minReplicas > 0 {
			replicas = minReplicas
		} else {
			replicas = -1
		}
	}

	if replicas == -1 {
		return nil
	} else {
		return &replicas
	}
}

func (nf *NuclioFunction) GetSpecMinReplicas() int32 {
	var minReplicas int32

	if nf.Spec.Replicas != nil {
		if *nf.Spec.Replicas <= 0 {
			minReplicas = 0
		} else {
			minReplicas = *nf.Spec.Replicas
		}
	} else {
		if nf.Spec.MinReplicas != nil {
			if *nf.Spec.MinReplicas <= 0 {
				minReplicas = 0
			} else {
				minReplicas = *nf.Spec.MinReplicas
			}
		} else {
			minReplicas = 0
		}
	}

	return minReplicas
}

func (nf *NuclioFunction) GetSpecMaxReplicas() int32 {
	var maxReplicas int32

	if nf.Spec.Replicas != nil {
		if *nf.Spec.Replicas <= 0 {
			maxReplicas = 0
		} else {
			maxReplicas = *nf.Spec.Replicas
		}
	} else {
		if nf.Spec.MaxReplicas != nil {
			if *nf.Spec.MaxReplicas <= 0 {
				maxReplicas = 0
			} else {
				maxReplicas = *nf.Spec.MaxReplicas
			}
		} else {
			maxReplicas = 10
		}
	}

	return maxReplicas
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuclioFunctionList is a list of NuclioFunction resources
type NuclioFunctionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NuclioFunction `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuclioProject describes a project.
type NuclioProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec platform.ProjectSpec `json:"spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuclioProjectList is a list of project resources
type NuclioProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NuclioProject `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuclioFunctionEvent describes a function event.
type NuclioFunctionEvent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec platform.FunctionEventSpec `json:"spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NuclioFunctionEventList is a list of functionevent resources
type NuclioFunctionEventList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NuclioFunctionEvent `json:"items"`
}
