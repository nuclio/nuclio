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

func (nf *NuclioFunction) GetComputedReplicas() *int32 {
	zero := int32(0)
	one := int32(1)

	if nf.Spec.Disable ||
		nf.Status.State == functionconfig.FunctionStateImported ||
		nf.Status.State == functionconfig.FunctionStateScaledToZero ||
		nf.Status.State == functionconfig.FunctionStateWaitingForScaleResourcesToZero {
		return &zero
	} else if nf.Spec.Replicas != nil {

		// Negative values -> 0
		if *nf.Spec.Replicas < 0 {
			return &zero
		} else {
			replicas := int32(*nf.Spec.Replicas)
			return &replicas
		}
	} else {

		// The user hasn't specified desired replicas
		// If the function doesn't have resources yet (creating/scaling up from zero) - base on the MinReplicas or default to 1
		if nf.Status.State == functionconfig.FunctionStateWaitingForResourceConfiguration ||
			nf.Status.State == functionconfig.FunctionStateWaitingForScaleResourcesFromZero {
			minReplicas := nf.GetComputedMinReplicas()

			if minReplicas > 0 {
				return &minReplicas
			} else {
				return &one
			}
		} else {

			// Should get here only in case of update of an existing deployment,
			// sending nil meaning leave the existing replicas as is
			return nil
		}
	}
}

func (nf *NuclioFunction) GetComputedMinReplicas() int32 {

	// Replicas takes precedence over MinReplicas, so if given override with its value
	if nf.Spec.Replicas != nil {

		// Negative values -> 0
		if *nf.Spec.Replicas < 0 {
			return 0
		} else {
			return int32(*nf.Spec.Replicas)
		}
	} else {
		if nf.Spec.MinReplicas != nil {

			// Negative values -> 0
			if *nf.Spec.MinReplicas < 0 {
				return 0
			} else {
				return int32(*nf.Spec.MinReplicas)
			}
		} else {

			// If neither Replicas nor MinReplicas is given, default to 1
			return 1
		}
	}
}

func (nf *NuclioFunction) GetComputedMaxReplicas() int32 {

	// Replicas takes precedence over MaxReplicas, so if given override with its value
	if nf.Spec.Replicas != nil {

		// Negative values -> 0
		if *nf.Spec.Replicas < 0 {
			return 0
		} else {
			return int32(*nf.Spec.Replicas)
		}
	} else {
		if nf.Spec.MaxReplicas != nil {

			// Negative values -> 0
			if *nf.Spec.MaxReplicas < 0 {
				return 0
			} else {
				return int32(*nf.Spec.MaxReplicas)
			}
		} else if nf.Spec.MinReplicas != nil {

			// If neither Replicas nor MaxReplicas is given, but MinReplicas is given, default to it (default to no HPA)
			return int32(*nf.Spec.MinReplicas)
		} else {

			// If neither Replicas nor MaxReplicas nor MinReplicas is given, default to 1
			return 1
		}
	}
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
