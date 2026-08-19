package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// streamAuth carries token renewal across the goroutines of a stream. The
// mutex plus the header comparison is what turns a burst of concurrent 401s
// into a single token fetch rather than one per in-flight line.
type streamAuth struct {
	provider   string
	tokenFile  string
	definition *AuthConfig
	mu         sync.Mutex
	header     string
	token      string
}

func (a *streamAuth) current() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.header
}

// renew replaces the token behind usedHeader. If another goroutine already
// replaced it, that result is reused instead of fetching a second time.
func (a *streamAuth) renew(usedHeader string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.header != usedHeader {
		return true
	}

	refreshed, err := reacquireToken(a.provider, a.tokenFile, a.token, a.definition)
	if err != nil {
		return false
	}
	a.token = refreshed.AccessToken
	a.header = authHeaderFor(refreshed)
	return true
}

// args: method url header concurrency maxTime
func runStream(args []string) {
	runStreamWithAuth(args, nil)
}

// runStreamWithAuth is runStream with an optional per-line token retry. A nil
// auth disables it for unauthenticated streams.
func runStreamWithAuth(args []string, auth *streamAuth) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: URL is required\n")
		os.Exit(1)
	}

	method := strings.ToUpper(args[0])
	if method == "" {
		method = "POST"
	}

	url := args[1]
	if url == "" {
		fmt.Fprintf(os.Stderr, "Error: URL is required\n")
		os.Exit(1)
	}

	var headers []string
	if len(args) > 2 && args[2] != "" {
		headers = strings.Split(args[2], "\n")
	}

	concurrency := 1
	if len(args) > 3 && args[3] != "" {
		fmt.Sscanf(args[3], "%d", &concurrency)
		if concurrency < 1 {
			concurrency = 1
		}
	}

	// --maxTime: per-request timeout in seconds (decimals allowed). 0 = no timeout.
	// A timeout surfaces as a transport error per line, keeping the resilient
	// {"error","input","status":0} contract (the stream still exits 0).
	maxTime := 0.0
	if len(args) > 4 && args[4] != "" {
		if v, err := strconv.ParseFloat(args[4], 64); err == nil && v > 0 {
			maxTime = v
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var mu sync.Mutex
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	client := &http.Client{}
	if maxTime > 0 {
		client.Timeout = time.Duration(maxTime * float64(time.Second))
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Validate JSON
		var jsonCheck json.RawMessage
		if err := json.Unmarshal([]byte(line), &jsonCheck); err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid JSON: %s\n", line)
			continue
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(body string) {
			defer wg.Done()
			defer func() { <-sem }()

			var status int
			var respBody []byte

			for attempt := 0; ; attempt++ {
				req, err := http.NewRequest(method, url, strings.NewReader(body))
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
					return
				}

				req.Header.Set("Content-Type", "application/json")
				for _, h := range headers {
					h = strings.TrimSpace(h)
					if h == "" {
						continue
					}
					parts := strings.SplitN(h, ":", 2)
					if len(parts) == 2 {
						req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
					}
				}

				// Read the auth header per attempt: another line may have
				// already renewed it while this request was in flight.
				authHeader := ""
				if auth != nil {
					authHeader = auth.current()
					parts := strings.SplitN(authHeader, ":", 2)
					if len(parts) == 2 {
						req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
					}
				}

				resp, err := client.Do(req)
				if err != nil {
					errJSON, _ := json.Marshal(map[string]interface{}{
						"error":  err.Error(),
						"input":  json.RawMessage(body),
						"status": 0,
					})
					mu.Lock()
					fmt.Fprintln(os.Stdout, string(errJSON))
					mu.Unlock()
					return
				}

				data, readErr := io.ReadAll(resp.Body)
				resp.Body.Close()
				if readErr != nil {
					errJSON, _ := json.Marshal(map[string]interface{}{
						"error":  readErr.Error(),
						"input":  json.RawMessage(body),
						"status": resp.StatusCode,
					})
					mu.Lock()
					fmt.Fprintln(os.Stdout, string(errJSON))
					mu.Unlock()
					return
				}

				// One retry on a rejected token. If renewal fails, the 401 is
				// reported as the line's result like any other status.
				if resp.StatusCode == http.StatusUnauthorized && auth != nil && attempt == 0 {
					if auth.renew(authHeader) {
						continue
					}
				}

				status = resp.StatusCode
				respBody = data
				break
			}

			// Try to parse response as JSON
			var respJSON json.RawMessage
			if err := json.Unmarshal(respBody, &respJSON); err != nil {
				// Response is not JSON, wrap it
				result, _ := json.Marshal(map[string]interface{}{
					"body":   string(respBody),
					"input":  json.RawMessage(body),
					"status": status,
				})
				mu.Lock()
				fmt.Fprintln(os.Stdout, string(result))
				mu.Unlock()
				return
			}

			// Response is JSON, wrap with metadata
			result, _ := json.Marshal(map[string]interface{}{
				"body":   respJSON,
				"input":  json.RawMessage(body),
				"status": status,
			})
			mu.Lock()
			fmt.Fprintln(os.Stdout, string(result))
			mu.Unlock()
		}(line)
	}

	wg.Wait()

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}
}
