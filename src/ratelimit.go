package src

import (
	"fmt"
	"sync"
	"time"
)

// Client-side throttling: the keyless explorers punish bursts with long 429
// penalties (Blockscout once answered "resets in 44 min"), and retrying after the
// fact only makes it worse. Spacing the requests ourselves keeps every host within
// its budget; all goroutines (analysis, expansions, lookups) share the same limiter.

// A budget is at most n requests in any window of length per.
type budget struct {
	n   int
	per time.Duration
}

// Conservative figures, below what each service documents or tolerates keyless.
// A host can have several budgets, e.g. BlockCypher: 3/s and 100/h.
var hostBudgets = map[string][]budget{
	"eth.blockscout.com":          {{3, time.Second}, {100, time.Minute}},
	"api.trongrid.io":             {{3, time.Second}},
	"blockstream.info":            {{5, time.Second}},
	"api.blockcypher.com":         {{3, time.Second}, {100, time.Hour}},
	"xrplcluster.com":             {{4, time.Second}},
	"api.mainnet-beta.solana.com": {{4, time.Second}, {80, 10 * time.Second}},
}

var defaultBudget = []budget{{5, time.Second}}

// Waiting longer than this for our own budget is not worth freezing the expansion:
// fail with the time left instead.
const maxThrottleWait = 20 * time.Second

type hostLimiter struct {
	mu      sync.Mutex
	sent    []time.Time // send times, oldest first, kept for the longest window
	blocked time.Time   // the server asked us to stop until then (long 429)
}

var (
	limitersMu sync.Mutex
	limiters   = map[string]*hostLimiter{}
	now        = time.Now
	sleep      = time.Sleep
)

func limiterFor(host string) *hostLimiter {
	limitersMu.Lock()
	defer limitersMu.Unlock()
	l, ok := limiters[host]
	if !ok {
		l = &hostLimiter{}
		limiters[host] = l
	}
	return l
}

func budgetsFor(host string) []budget {
	if b, ok := hostBudgets[host]; ok {
		return b
	}
	return defaultBudget
}

// Blocks until a request to host fits every budget, then books it.
func throttle(host string) error {
	l := limiterFor(host)
	budgets := budgetsFor(host)
	for {
		l.mu.Lock()
		t := now()
		if t.Before(l.blocked) {
			left := l.blocked.Sub(t)
			l.mu.Unlock()
			return rateLimitError(host, left, true)
		}
		wait := l.waitLocked(t, budgets)
		if wait <= 0 {
			l.sent = append(l.sent, t)
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()
		if wait > maxThrottleWait {
			return fmt.Errorf("%s: request budget used up (EyeTrace keeps under the explorer's limit), retry in about %s",
				host, wait.Round(time.Second))
		}
		sleep(wait)
	}
}

// How long until one more request fits all budgets (0 = now). Also forgets send
// times older than the longest window.
func (l *hostLimiter) waitLocked(t time.Time, budgets []budget) time.Duration {
	var longest time.Duration
	for _, b := range budgets {
		if b.per > longest {
			longest = b.per
		}
	}
	drop := 0
	for drop < len(l.sent) && t.Sub(l.sent[drop]) >= longest {
		drop++
	}
	l.sent = l.sent[drop:]

	var wait time.Duration
	for _, b := range budgets {
		// requests inside this window
		inWindow := 0
		for i := len(l.sent) - 1; i >= 0 && t.Sub(l.sent[i]) < b.per; i-- {
			inWindow++
		}
		if inWindow < b.n {
			continue
		}
		// the oldest of the last n must leave the window first
		oldest := l.sent[len(l.sent)-b.n]
		if w := oldest.Add(b.per).Sub(t); w > wait {
			wait = w
		}
	}
	return wait
}

// After a 429 whose reset is too far to wait for, every further request would hit
// the server for nothing and, on some explorers, extend the penalty.
func blockHost(host string, d time.Duration) {
	l := limiterFor(host)
	l.mu.Lock()
	if until := now().Add(d); until.After(l.blocked) {
		l.blocked = until
	}
	l.mu.Unlock()
}
