package wsc

import (
	"net/http"
	"time"
)

type Options struct {
	Header http.Header

	KeepAlive bool
	Reconnect time.Duration
	Backoff   time.Duration

	FnReceive FnReceive

	SendTimeout time.Duration

	fnLog FnLog
}

type FnWithOption func(opt *Options)

// WithKeepAlive specifies that the connection must be kept alive and reconnect after a disconnect
func WithKeepAlive(reconnect time.Duration, backoff time.Duration) FnWithOption {
	return func(opt *Options) {
		opt.KeepAlive = true
		opt.Reconnect = reconnect
		opt.Backoff = backoff
	}
}

// WithFnReceive delegate to handle data received over the WebSocket
func WithFnReceive(fnReceive FnReceive) FnWithOption {
	return func(opt *Options) {
		opt.FnReceive = fnReceive
	}
}

// WithSendTimeout maximum wait time for sending message. Default is 10 seconds.
func WithSendTimeout(td time.Duration) FnWithOption {
	return func(opt *Options) {
		opt.SendTimeout = td
	}
}

// WithLogger logging feedback from connection
func WithLogger(log FnLog) FnWithOption {
	return func(opt *Options) {
		opt.fnLog = log
	}
}

func WithHeader(h http.Header) FnWithOption {
	return func(opt *Options) {
		opt.Header = h
	}
}
