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
	var replicas *int32
	zero := int32(0)
	one := int32(1)

	if nf.Spec.Disabled || nf.Status.State == functionconfig.FunctionStateScaledToZero {
		replicas = &zero
	} else if nf.Spec.Replicas != nil {

		// Negative values -> 0
		if *nf.Spec.Replicas < 0 {
			replicas = &zero
		} else {
			replicas = nf.Spec.Replicas
		}
	} else {

		// If the user hasn't specified desired replicas - base on the MinReplicas
		minReplicas := nf.GetSpecMinReplicas()

		if minReplicas > 0 {
			replicas = &minReplicas
		} else {

			// If the function doesn't have resources yet (creating/scaling up from zero) - start from 1
			if nf.Status.State == functionconfig.FunctionStateWaitingForResourceConfiguration {
				replicas = &one
			} else {

				// Should get here only in case of update of an existing deployment,
				// sending nil meaning leave the existing replicas as is
				replicas = nil
			}
		}
	}

	return replicas

}

func (nf *NuclioFunction) GetSpecMinReplicas() int32 {
	var minReplicas int32

	// Replicas takes precedence over MinReplicas, so if given override with its value
	if nf.Spec.Replicas != nil {

		// Negative values -> 0
		if *nf.Spec.Replicas < 0 {
			minReplicas = 0
		} else {
			minReplicas = *nf.Spec.Replicas
		}
	} else {
		if nf.Spec.MinReplicas != nil {

			// Negative values -> 0
			if *nf.Spec.MinReplicas < 0 {
				minReplicas = 0
			} else {
				minReplicas = *nf.Spec.MinReplicas
			}
		} else {

			// If neither Replicas nor MinReplicas is given, default to 1
			minReplicas = 1
		}
	}

	return minReplicas
}

func (nf *NuclioFunction) GetSpecMaxReplicas() int32 {
	var maxReplicas int32

	// Replicas takes precedence over MaxReplicas, so if given override with its value
	if nf.Spec.Replicas != nil {

		// Negative values -> 0
		if *nf.Spec.Replicas <= 0 {
			maxReplicas = 0
		} else {
			maxReplicas = *nf.Spec.Replicas
		}
	} else {
		if nf.Spec.MaxReplicas != nil {

			// Negative values -> 0
			if *nf.Spec.MaxReplicas <= 0 {
				maxReplicas = 0
			} else {
				maxReplicas = *nf.Spec.MaxReplicas
			}
		} else {

			// If neither Replicas nor MaxReplicas is given, default to 1
			maxReplicas = 1
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
