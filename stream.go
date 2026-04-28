package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// args: method url header concurrency
func runStream(args []string) {
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

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var mu sync.Mutex
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	client := &http.Client{}

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
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				errJSON, _ := json.Marshal(map[string]interface{}{
					"error":  err.Error(),
					"input":  json.RawMessage(body),
					"status": resp.StatusCode,
				})
				mu.Lock()
				fmt.Fprintln(os.Stdout, string(errJSON))
				mu.Unlock()
				return
			}

			// Try to parse response as JSON
			var respJSON json.RawMessage
			if err := json.Unmarshal(respBody, &respJSON); err != nil {
				// Response is not JSON, wrap it
				result, _ := json.Marshal(map[string]interface{}{
					"body":   string(respBody),
					"input":  json.RawMessage(body),
					"status": resp.StatusCode,
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
				"status": resp.StatusCode,
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
