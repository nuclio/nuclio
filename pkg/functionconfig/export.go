package functionconfig

import "strconv"

func CleanFunctionSpec(functionConfig *Config) {

	// artifacts are created unique to the cluster not needed to be returned to any client of nuclio REST API
	functionConfig.Spec.RunRegistry = ""
	functionConfig.Spec.Build.Registry = ""
	if functionConfig.Spec.Build.FunctionSourceCode != "" {
		functionConfig.Spec.Image = ""
	}
}

func PrepareFunctionForExport(functionConfig *Config, noScrub bool) {
	if !noScrub {
		scrubFunctionData(functionConfig)
	}
	addSkipAnnotations(functionConfig)
}

func addSkipAnnotations(functionConfig *Config) {

	if functionConfig.Meta.Annotations == nil {
		functionConfig.Meta.Annotations = map[string]string{}
	}

	// add annotations for not deploying or building on import
	functionConfig.Meta.Annotations[FunctionAnnotationSkipBuild] = strconv.FormatBool(true)
	functionConfig.Meta.Annotations[FunctionAnnotationSkipDeploy] = strconv.FormatBool(true)
}

func scrubFunctionData(functionConfig *Config) {
	CleanFunctionSpec(functionConfig)

	// scrub namespace from function meta
	functionConfig.Meta.Namespace = ""

	// remove secrets and passwords from triggers
	newTriggers := functionConfig.Spec.Triggers
	for triggerName, trigger := range newTriggers {
		trigger.Password = ""
		trigger.Secret = ""
		newTriggers[triggerName] = trigger
	}
	functionConfig.Spec.Triggers = newTriggers
}
