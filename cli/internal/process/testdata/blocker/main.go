//go:build ignore

// blocker is a test helper that sleeps for up to 60 seconds.
// It exits if stdin is closed, but also exits after 60s regardless.
// Used by supervisor_windows_test.go to provide a stable long-running
// process without spawning shell children.
package main

import (
	"time"
)

func main() {
	time.Sleep(60 * time.Second)
}
