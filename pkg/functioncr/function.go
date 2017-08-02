package functioncr

import (
	"strconv"
	"regexp"
	"strings"

	"fmt"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/pkg/errors"
)

// allow alphanumeric (inc. underscore) and hyphen
var nameValidator = regexp.MustCompile(`^[\w\-]+$`).MatchString

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

func (f *Function) GetNameAndVersion() (name string, version int, err error) {
	name = f.Name
	version = 0

	// verify name has only alphanumeric characters, underscores and hyphens
	if !nameValidator(f.Name) {
		err = errors.New("Name is invalid. Must only contain alphanumeric (inc. underscore) and hyphen")
		return
	}

	if lastHyphenIdx := strings.LastIndex(name, "-"); lastHyphenIdx > 0 {

		// get the string that follows the last hyphen
		version, err = strconv.Atoi(name[lastHyphenIdx+1:])
		if err != nil {
			return
		}

		name = name[:lastHyphenIdx]
	}

	return name, version, nil
}

func (f *Function) GetNamespacedName() string {
	return fmt.Sprintf("%s.%s", f.Namespace, f.Name)
}
