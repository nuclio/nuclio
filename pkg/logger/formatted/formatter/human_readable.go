package formatter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/mgutz/ansi"
	"github.com/sirupsen/logrus"
)

const reset = ansi.Reset

type HumanReadable struct{}

func (f *HumanReadable) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}
	f.printColored(b, entry)
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func (f *HumanReadable) printColored(b *bytes.Buffer, entry *logrus.Entry) {
	severityColor := getSeverityColor(entry.Level)
	severity := getSeverityShortcut(entry.Level)
	formattedWho := getFormattedWho(entry.Data)

	fmt.Fprintf(b, "%s%s%s %s%s%s: %s(%+s)%s %s",
		ansi.LightBlack, entry.Time.Format("02.01.06 15:04:05.000000"), reset,
		ansi.Cyan, formattedWho, reset,
		severityColor, severity, reset,
		entry.Message)

	additionalInfo := make([]string, 0, len(entry.Data)-1)
	for k := range entry.Data {
		if k != "who" {
			keyValue := fmt.Sprintf("%s%s%s=%+v", severityColor, k, reset, entry.Data[k])
			additionalInfo = append(additionalInfo, keyValue)
		}
	}

	formattedAdditionalInfo := strings.Join(additionalInfo, ", ")

	if len(formattedAdditionalInfo) > 0 {
		openBracket := fmt.Sprint(ansi.Cyan, fmt.Sprintf(" {%s", reset))
		closeBracket := fmt.Sprint(ansi.Cyan, "}")
		fmt.Fprint(b, openBracket, formattedAdditionalInfo, closeBracket)
	}
}

func getFormattedWho(data logrus.Fields) string {
	who, ok := data["who"]
	if ok {
		whoStr := fmt.Sprintf("%30s", who)
		return fmt.Sprintf("%30s", whoStr[len(whoStr)-30:])
	}

	return ""
}

func getSeverityColor(l logrus.Level) string {
	switch l {
	case logrus.InfoLevel:
		return ansi.Green
	case logrus.WarnLevel:
		return ansi.Yellow
	case logrus.ErrorLevel:
		return ansi.Red
	}

	return ansi.Blue
}

func getSeverityShortcut(l logrus.Level) string {
	switch l {
	case logrus.InfoLevel:
		return "I"
	case logrus.WarnLevel:
		return "W"
	case logrus.DebugLevel:
		return "D"
	}

	return "E"
}
