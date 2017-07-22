package nuclio

type Response struct {
	StatusCode  int
	ContentType string
	Headers     map[string]string
	Body        []byte
}
