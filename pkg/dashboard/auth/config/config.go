package config

import (
	"time"

	"github.com/nuclio/nuclio/pkg/dashboard/auth"
)

type AuthContextKey string

const IguazioContextKey AuthContextKey = "IguazioAuth"

type Iguazio struct {
	Timeout         time.Duration
	VerificationURL string
}

type Options struct {
	Kind    auth.Kind
	Iguazio *Iguazio
}

func NewOptions(kind auth.Kind) *Options {
	opts := &Options{
		Kind: kind,
	}
	if kind == auth.KindIguazio {
		opts.Iguazio = &Iguazio{}
	}
	return opts
}
