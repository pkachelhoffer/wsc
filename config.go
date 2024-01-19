package wsc

import "time"

const (
	defaultSendTimeout   = time.Second * 10
	maxBackoff           = 10               // Max backoff is the backoff duration multiplied by this setting
	waitForStatusTimeout = time.Second * 10 // Time to wait for a requested status
)
