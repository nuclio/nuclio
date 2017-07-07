package app

import "github.com/nuclio/nuclio/pkg/logger"

type Controller struct {
	logger logger.Logger
	controlMessageChan chan controlMessage
}

func NewController