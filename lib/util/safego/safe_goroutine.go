package safego

import (
	"runtime/debug"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var recoveredPanicsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "goone_safego_recovered_panics_total",
	Help: "Total panics recovered by safego.",
})

// Go runs f in a new goroutine, recovering from panics.
func Go(f func()) {
	if f == nil {
		return
	}

	go SafeFunc(f)
}

// SafeFunc runs f, recovering from panics. A panic is logged and counted but
// never terminates the process; crashing the whole service is a decision that
// must be made explicitly by the caller, not by a recovery helper.
func SafeFunc(f func()) {
	defer func() {
		if r := recover(); r != nil {
			recoveredPanicsTotal.Inc()
			logger.ErrorDepthf(2, "recovered panic: %v\n%s", r, debug.Stack())
		}
	}()
	f()
}
