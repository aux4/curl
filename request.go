package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// args: method url header body showHeaders
func runRequest(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: URL is required\n")
		os.Exit(1)
	}

	method := strings.ToUpper(args[0])
	if method == "" {
		method = "GET"
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

	body := ""
	if len(args) > 3 {
		body = args[3]
	}

	showHeaders := false
	if len(args) > 4 && args[4] == "true" {
		showHeaders = true
	}

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err == nil && len(data) > 0 {
				reqBody = bytes.NewReader(data)
			}
		}
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		os.Exit(1)
	}

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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if showHeaders {
		fmt.Fprintf(os.Stdout, "HTTP/%d.%d %s\n", resp.ProtoMajor, resp.ProtoMinor, resp.Status)
		for key, values := range resp.Header {
			for _, v := range values {
				fmt.Fprintf(os.Stdout, "%s: %s\n", key, v)
			}
		}
		fmt.Fprintln(os.Stdout)
	}

	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading response: %v\n", err)
		os.Exit(1)
	}
}
