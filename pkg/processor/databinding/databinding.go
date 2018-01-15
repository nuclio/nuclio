package databinding

import "github.com/nuclio/nuclio-sdk"

type DataBinding interface{}

type AbstractDataBinding struct {
	Logger nuclio.Logger
}
