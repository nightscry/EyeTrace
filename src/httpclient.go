package src

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// http.Get uses a client with no timeout: one explorer that never answers
// would leave the analysis goroutine hanging forever.
var httpClient = &http.Client{Timeout: 20 * time.Second}

// Keyless explorers rate-limit hard (429): a few spaced retries usually get through,
// which matters when expanding nodes fires many requests in a row. When the server
// says the limit resets later than maxRetryWait, waiting is pointless: fail at once
// and tell the user how long it is.
const (
	maxRetries   = 3
	maxRetryWait = 15 * time.Second
)

func getJSON(url string, out any) error {
	return doJSON(http.MethodGet, url, nil, out)
}

// JSON-RPC chains (XRPL, Solana) talk POST with a JSON body.
func postJSON(url string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return doJSON(http.MethodPost, url, b, out)
}

func doJSON(method, url string, body []byte, out any) error {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequest(method, url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("User-Agent", "EyeTrace/"+Version)

		if err := throttle(req.URL.Host); err != nil {
			return err
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			wait, known := rateLimitWait(resp)
			resp.Body.Close()
			if attempt < maxRetries && (!known || wait <= maxRetryWait) {
				if !known {
					wait = time.Second << attempt // 1s, 2s, 4s
				}
				sleep(wait)
				continue
			}
			if known {
				blockHost(req.URL.Host, wait)
			}
			return rateLimitError(req.URL.Host, wait, known)
		}

		err = decodeResponse(resp, out)
		resp.Body.Close()
		return err
	}
}

// How long the server asks to wait: standard Retry-After (seconds), or Blockscout's
// x-ratelimit-reset (milliseconds until the window resets).
func rateLimitWait(resp *http.Response) (time.Duration, bool) {
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
		return time.Duration(s) * time.Second, true
	}
	if ms, err := strconv.ParseInt(resp.Header.Get("X-Ratelimit-Reset"), 10, 64); err == nil && ms > 0 {
		return time.Duration(ms) * time.Millisecond, true
	}
	return 0, false
}

func rateLimitError(host string, wait time.Duration, known bool) error {
	if !known || wait < time.Minute {
		return fmt.Errorf("%s: rate limited (HTTP 429), retry in a few seconds", host)
	}
	return fmt.Errorf("%s: rate limited (HTTP 429), the limit resets in about %d min", host, int(wait.Round(time.Minute).Minutes()))
}

// Without the status check an explorer's error body would surface as a
// confusing JSON decode error.
func decodeResponse(resp *http.Response, out any) error {
	host := resp.Request.URL.Host

	if resp.StatusCode != http.StatusOK {
		// a short excerpt of the body is usually the explorer's own error message
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("%s: HTTP %d %s", host, resp.StatusCode, body)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s: unexpected response: %w", host, err)
	}
	return nil
}
