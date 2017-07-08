package functioncr

import (
	"strconv"
	"strings"

	"fmt"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Function struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               FunctionSpec   `json:"spec"`
	Status             FunctionStatus `json:"status,omitempty"`
}

func (f *Function) SetStatus(state FunctionState, message string) {
	f.Status.ObservedGen = f.ResourceVersion
	f.Status.State = state
	f.Status.Message = message
}

func (f *Function) GetLabels() map[string]string {
	if f.ObjectMeta.Labels == nil {
		f.ObjectMeta.Labels = make(map[string]string)
	}

	return f.Labels
}

func (f *Function) GetNameAndVersion() (string, int) {
	functionName := f.Name
	functionVersion := 0

	if lastHyphenIdx := strings.LastIndex(functionName, "-"); lastHyphenIdx > 0 {

		// get the string that follows the last hyphen
		functionVersion, err := strconv.Atoi(functionName[lastHyphenIdx+1:])
		if err == nil && functionVersion > 0 && f.Spec.Version > 0 && f.Labels["function"] == functionName[:lastHyphenIdx] {
			functionName = functionName[:lastHyphenIdx]
		}
	}

	return functionName, functionVersion
}

func (f *Function) GetNamespacedName() string {
	return fmt.Sprintf("%s.%s", f.Namespace, f.Name)
}
