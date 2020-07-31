package sfu

import "errors"

var (
	errChanClosed         = errors.New("channel closed")
	errPtNotSupported     = errors.New("payload type not supported")
	errMethodNotSupported = errors.New("method not supported")
	errReceiverClosed     = errors.New("receiver closed")
)
