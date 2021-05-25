package daemon

import (
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var signalOnce = new(sync.Once)
var ch = make(chan os.Signal, 8)

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
	done := false
	signalOnce.Do(func() {
		signals := make([]os.Signal, 0, len(handlers))
		for sig := range handlers {
			signals = append(signals, sig)
		}

		signal.Notify(ch, signals...)
	loop:
		for sig := range ch {
			for _, f := range handlers[sig] {
				err = f(sig)
				if err == ErrStop {
					break loop
				}
				if err != nil {
					break
				}
			}
		}

		signal.Stop(ch)
		done = true
	})

	if done == false {
		signal.Stop(ch)
		signalOnce = new(sync.Once)
		return ServeSignals()
	}

	return
}

var handlers = make(map[os.Signal][]SignalHandlerFunc)

func init() {
	handlers[syscall.SIGTERM] = []SignalHandlerFunc{sigtermDefaultHandler}
}

func sigtermDefaultHandler(sig os.Signal) error {
	return ErrStop
}
