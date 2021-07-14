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
	Mode    auth.Mode `json:"mode"`
	Iguazio *Iguazio
}

func NewOptions(mode auth.Mode) *Options {
	opts := &Options{
		Mode: mode,
	}
	if mode == auth.ModeIguazio {
		opts.Iguazio = &Iguazio{}
	}

	return opts
}
