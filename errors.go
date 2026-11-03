package main

import (
	"errors"
	"log"
	"strings"
)

// errTryAgain is what a person sees when something broke on our side.
var errTryAgain = errors.New("Something went wrong. Please try again in a moment.")

// internalFailure logs the real cause and returns the message that is safe to
// show: database, network and other technical details stay in the log.
func internalFailure(what string, err error) error {
	log.Printf("❌ %s: %v", what, err)
	return errTryAgain
}

// technicalMarkers are fragments that only appear in errors from the
// database, the network or the runtime, never in messages written for people.
var technicalMarkers = []string{
	"sqlstate", "pgx", "pgconn", "error:", "violates", "duplicate key",
	"redis", "dial tcp", "connection refused", "connection reset", "i/o timeout",
	"no such host", "context deadline", "context canceled", "broken pipe",
	"unexpected eof", "internal error", "runtime error", "nil pointer",
}

// publicError returns the text of err for showing to a person. Messages that
// look technical are logged and replaced by errTryAgain.
func publicError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	for _, m := range technicalMarkers {
		if strings.Contains(low, m) {
			log.Printf("❌ hidden from the user: %s", msg)
			return errTryAgain.Error()
		}
	}
	return msg
}
