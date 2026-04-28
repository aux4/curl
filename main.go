package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: aux4-curl <command> [options]\n")
		fmt.Fprintf(os.Stderr, "Commands: request, stream\n")
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "request":
		runRequest(os.Args[2:])
	case "stream":
		runStream(os.Args[2:])
	case "oauth-login":
		runOAuthLogin(os.Args[2:])
	case "oauth-token":
		runOAuthToken(os.Args[2:])
	case "oauth-status":
		runOAuthStatus(os.Args[2:])
	case "oauth-logout":
		runOAuthLogout(os.Args[2:])
	case "auth-request":
		runAuthRequest(os.Args[2:])
	case "auth-stream":
		runAuthStream(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		fmt.Fprintf(os.Stderr, "Available commands: request, stream, oauth-login, oauth-token, oauth-status, oauth-logout, auth-request, auth-stream\n")
		os.Exit(1)
	}
}
