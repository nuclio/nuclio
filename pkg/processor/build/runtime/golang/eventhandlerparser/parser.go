package eventhandlerparser

// EventHandlerParser finds event handlers
type EventHandlerParser interface {
	ParseEventHandlers(identifier string) ([]string, []string, error)
}
