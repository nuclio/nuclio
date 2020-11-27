package common

type CatchAndLogPanicOptions struct {
	Args          []interface{}
	CustomHandler func(error)
}
