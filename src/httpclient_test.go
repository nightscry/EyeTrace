package src

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRateLimitLongWaitFailsFast(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("X-Ratelimit-Reset", "2650603") // Blockscout, in ms: ~44 min
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	start := time.Now()
	var out any
	err := getJSON(srv.URL, &out)
	if err == nil || !strings.Contains(err.Error(), "about 44 min") {
		t.Fatalf("err = %v", err)
	}
	if calls != 1 || time.Since(start) > time.Second {
		t.Fatalf("must not retry a 44 min wait: %d calls in %v", calls, time.Since(start))
	}
}

func TestRateLimitShortWaitRetries(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`{"ok": true}`))
	}))
	defer srv.Close()

	var out struct{ OK bool }
	if err := getJSON(srv.URL, &out); err != nil || !out.OK || calls != 2 {
		t.Fatalf("err=%v out=%+v calls=%d", err, out, calls)
	}
}

func TestHTTPErrorShowsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "address not indexed", http.StatusBadGateway)
	}))
	defer srv.Close()

	var out any
	err := getJSON(srv.URL, &out)
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") || !strings.Contains(err.Error(), "address not indexed") {
		t.Fatalf("err = %v", err)
	}
}

// fake clock: sleeping advances it, so the budgets are checked without real waits
func fakeClock(t *testing.T) *time.Time {
	clock := time.Unix(1700000000, 0)
	oldNow, oldSleep := now, sleep
	now = func() time.Time { return clock }
	sleep = func(d time.Duration) { clock = clock.Add(d) }
	t.Cleanup(func() { now, sleep = oldNow, oldSleep })
	return &clock
}

func TestThrottleSpacesRequests(t *testing.T) {
	clock := fakeClock(t)
	host := "throttle.test"
	hostBudgets[host] = []budget{{3, time.Second}, {5, time.Minute}}
	delete(limiters, host)
	defer delete(hostBudgets, host)

	start := *clock
	var at []time.Duration
	for i := 0; i < 5; i++ {
		if err := throttle(host); err != nil {
			t.Fatal(err)
		}
		at = append(at, clock.Sub(start))
	}
	// 3 at once, then the 4th and 5th wait for the 1 s window to roll over
	if at[2] != 0 || at[3] != time.Second || at[4] != time.Second {
		t.Fatalf("send times: %v", at)
	}
	// the 6th would need the 1 min window: longer than maxThrottleWait, fail with the time
	err := throttle(host)
	if err == nil || !strings.Contains(err.Error(), "retry in about 59s") {
		t.Fatalf("err = %v", err)
	}
}

func TestLong429BlocksHost(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	var out any
	getJSON(srv.URL, &out)
	err := getJSON(srv.URL, &out)
	if calls != 1 || err == nil || !strings.Contains(err.Error(), "about 10 min") {
		t.Fatalf("second request must not reach the server: calls=%d err=%v", calls, err)
	}
}
