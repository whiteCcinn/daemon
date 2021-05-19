package daemon

import (
	"errors"
	"os"
	"os/signal"
	"syscall"
)

// ErrStop should be returned signal handler function
// for termination of handling signals.
var ErrStop = errors.New("stop serve signals")

// SignalHandlerFunc is the interface for signal handler functions.
type SignalHandlerFunc func(sig os.Signal) (err error)

// SetSigHandler sets handler for the given signals.
// SIGTERM has the default handler, he returns ErrStop.
func SetSigHandler(handler SignalHandlerFunc, signals ...os.Signal) {
	for _, sig := range signals {
		handlers[sig] = append(handlers[sig], handler)
	}
}

// ServeSignals calls handlers for system signals.
func ServeSignals() (err error) {
	signals := make([]os.Signal, 0, len(handlers))
	for sig := range handlers {
		signals = append(signals, sig)
	}

	ch := make(chan os.Signal, 8)
	signal.Notify(ch, signals...)

	for sig := range ch {
		for _, f := range handlers[sig] {
			err = f(sig)
			if err == ErrStop {
				err = nil
			}
			if err != nil {
				break
			}
		}
	}

	signal.Stop(ch)

	return
}

var handlers = make(map[os.Signal][]SignalHandlerFunc)

func init() {
	handlers[syscall.SIGTERM] = []SignalHandlerFunc{sigtermDefaultHandler}
}

func sigtermDefaultHandler(sig os.Signal) error {
	return ErrStop
}
