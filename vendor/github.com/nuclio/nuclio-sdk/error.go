package nuclio

type WithStatusCode interface {
	StatusCode() int
}

type ErrorWithStatusCode struct {
	error
	statusCode int
}

func (ewsc *ErrorWithStatusCode) StatusCode() int {
	return ewsc.statusCode
}

// wrapped errors
var ErrCreated = ErrorWithStatusCode{statusCode: 201}
var ErrAccepted = ErrorWithStatusCode{statusCode: 202}
var ErrNonAuthoritativeInfo = ErrorWithStatusCode{statusCode: 203}
var ErrNoContent = ErrorWithStatusCode{statusCode: 204}
var ErrResetContent = ErrorWithStatusCode{statusCode: 205}
var ErrPartialContent = ErrorWithStatusCode{statusCode: 206}
var ErrMultiStatus = ErrorWithStatusCode{statusCode: 207}
var ErrAlreadyReported = ErrorWithStatusCode{statusCode: 208}
var ErrIMUsed = ErrorWithStatusCode{statusCode: 226}

var ErrMultipleChoices = ErrorWithStatusCode{statusCode: 300}
var ErrMovedPermanently = ErrorWithStatusCode{statusCode: 301}
var ErrFound = ErrorWithStatusCode{statusCode: 302}
var ErrSeeOther = ErrorWithStatusCode{statusCode: 303}
var ErrNotModified = ErrorWithStatusCode{statusCode: 304}
var ErrUseProxy = ErrorWithStatusCode{statusCode: 305}
var ErrTemporaryRedirect = ErrorWithStatusCode{statusCode: 307}
var ErrPermanentRedirect = ErrorWithStatusCode{statusCode: 308}

var ErrBadRequest = ErrorWithStatusCode{statusCode: 400}
var ErrUnauthorized = ErrorWithStatusCode{statusCode: 401}
var ErrPaymentRequired = ErrorWithStatusCode{statusCode: 402}
var ErrForbidden = ErrorWithStatusCode{statusCode: 403}
var ErrNotFound = ErrorWithStatusCode{statusCode: 404}
var ErrMethodNotAllowed = ErrorWithStatusCode{statusCode: 405}
var ErrNotAcceptable = ErrorWithStatusCode{statusCode: 406}
var ErrProxyAuthRequired = ErrorWithStatusCode{statusCode: 407}
var ErrRequestTimeout = ErrorWithStatusCode{statusCode: 408}
var ErrConflict = ErrorWithStatusCode{statusCode: 409}
var ErrGone = ErrorWithStatusCode{statusCode: 410}
var ErrLengthRequired = ErrorWithStatusCode{statusCode: 411}
var ErrPreconditionFailed = ErrorWithStatusCode{statusCode: 412}
var ErrRequestEntityTooLarge = ErrorWithStatusCode{statusCode: 413}
var ErrRequestURITooLong = ErrorWithStatusCode{statusCode: 414}
var ErrUnsupportedMediaType = ErrorWithStatusCode{statusCode: 415}
var ErrRequestedRangeNotSatisfiable = ErrorWithStatusCode{statusCode: 416}
var ErrExpectationFailed = ErrorWithStatusCode{statusCode: 417}
var ErrTeapot = ErrorWithStatusCode{statusCode: 418}
var ErrUnprocessableEntity = ErrorWithStatusCode{statusCode: 422}
var ErrLocked = ErrorWithStatusCode{statusCode: 423}
var ErrFailedDependency = ErrorWithStatusCode{statusCode: 424}
var ErrUpgradeRequired = ErrorWithStatusCode{statusCode: 426}
var ErrPreconditionRequired = ErrorWithStatusCode{statusCode: 428}
var ErrTooManyRequests = ErrorWithStatusCode{statusCode: 429}
var ErrRequestHeaderFieldsTooLarge = ErrorWithStatusCode{statusCode: 431}
var ErrUnavailableForLegalReasons = ErrorWithStatusCode{statusCode: 451}

var ErrInternalServerError = ErrorWithStatusCode{statusCode: 500}
var ErrNotImplemented = ErrorWithStatusCode{statusCode: 501}
var ErrBadGateway = ErrorWithStatusCode{statusCode: 502}
var ErrServiceUnavailable = ErrorWithStatusCode{statusCode: 503}
var ErrGatewayTimeout = ErrorWithStatusCode{statusCode: 504}
var ErrHTTPVersionNotSupported = ErrorWithStatusCode{statusCode: 505}
var ErrInsufficientStorage = ErrorWithStatusCode{statusCode: 507}
var ErrLoopDetected = ErrorWithStatusCode{statusCode: 508}
var ErrNotExtended = ErrorWithStatusCode{statusCode: 510}
var ErrNetworkAuthenticationRequired = ErrorWithStatusCode{statusCode: 511}
