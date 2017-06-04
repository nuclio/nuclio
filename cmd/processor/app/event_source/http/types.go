package http

type Response struct {
	StatusCode  int
	ContentType string
	Header      map[string]string
	Body        []byte
}
