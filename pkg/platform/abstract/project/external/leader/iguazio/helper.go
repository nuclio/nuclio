package iguazio

func labelListToMap(labelList []Label) map[string]string {
	labelsMap := map[string]string{}

	for _, label := range labelList {
		labelsMap[label.Name] = label.Value
	}

	return labelsMap
}

func labelMapToList(labelMap map[string]string) []Label {
	var labelList []Label

	for labelName, labelValue := range labelMap {
		labelList = append(labelList, Label{Name: labelName, Value: labelValue})
	}

	return labelList
}
