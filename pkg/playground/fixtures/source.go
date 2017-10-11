package fixtures

// Sources contains a map of built in source fixtures
var Sources = map[string]string{
	"echo.go": `package echo

import (
	"github.com/nuclio/nuclio-sdk"
)

// Echo will reply with whatever you POST to it
func Echo(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	context.Logger.Info("Echoing body")

	return nuclio.Response{
		StatusCode:  200,
		ContentType: "application/text",
		Body:		 []byte(event.GetBody()),
	}, nil
}
`,
	"responder.py": `def handler(context, event):
	context.logger.info('Responding')

	# return a response, where the body is some environment variable and headers is another
	return context.Response(body='Response: {0}'.format(os.environ.get('ENV_VAR1'),
							headers={'x-env-var-2': os.environ.get('ENV_VAR2')},
							content_type='text/plain',
							status_code=200)
`,
}
