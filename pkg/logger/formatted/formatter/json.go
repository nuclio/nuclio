package formatter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

type Json struct{}

func (f *Json) Format(entry *logrus.Entry) ([]byte, error) {
	data := make(logrus.Fields, len(entry.Data)+6)

	for k, v := range entry.Data {
		switch v := v.(type) {
		case error:

			// Otherwise errors are ignored by `encoding/json`
			// https://github.com/Sirupsen/logrus/issues/137
			data[k] = v.Error()
		default:
			data[k] = v
		}
	}

	// "when": "2016-06-19T09:56:29.043641"
	data["when"] = entry.Time.Format("2006-01-02T15:04:05.000000")

	// "who": "access_control"
	data["who"] = entry.Data["who"]

	// "severity": "DEBUG"
	data["severity"] = strings.ToUpper(entry.Level.String())

	// "what": "Using etcd discovery"
	data["what"] = entry.Message

	// "more": "{\"etcd_address\": \"127.0.0.1:5251\"}"
	data["more"] = buildMoreValue(&data)

	// "lang": "go"
	data["lang"] = "go"

	// extract context as first-class citizen
	ctx, ok := entry.Data["ctx"]
	if !ok {
		ctx = ""
	}

	// "ctx": "some-uuid"
	data["ctx"] = ctx

	serialized, err := json.Marshal(data)

	if err != nil {
		return nil, fmt.Errorf("Failed to marshal fields to JSON, %v", err)
	}

	// we append the rune (byte) '\n' rather than the string "\n"
	return append(serialized, '\n'), nil
}

// Build data["more"] value
func buildMoreValue(data *logrus.Fields) map[string]string {
	additionalData := make(map[string]string, 0)

	for key, value := range *data {
		switch key {
		case "when":
		case "who":
		case "severity":
		case "what":
		case "more":
		case "ctx":
			// don't include these inside the more value
		default:

			formattedValue := convertValueToString(value)
			additionalData[key] = formattedValue

			// The key was copied to additional_data (No need for duplication)
			delete(*data, key)
		}
	}

	return additionalData
}

// Convert the given value to string
func convertValueToString(value interface{}) string {
	switch value := value.(type) {
	case string:
		return value
	case error:

		//return error message
		return value.Error()
	default:
		return fmt.Sprintf("%v", value)
	}
}
