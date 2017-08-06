package functioncr

import "fmt"

// returns true if function was processed
func WaitConditionProcessed(functioncrInstance *Function) (bool, error) {

	// TODO: maybe possible that error existed before and our new post wasnt yet updated to status created ("")
	if functioncrInstance.Status.State != FunctionStateCreated {
		if functioncrInstance.Status.State == FunctionStateError {
			return true, fmt.Errorf("Function in error state (%s)", functioncrInstance.Status.Message)
		}

		return true, nil
	}

	return false, nil
}
