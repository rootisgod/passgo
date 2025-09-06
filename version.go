package main

import (
	"fmt"
	"runtime"
)

// Version information
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// GetVersion returns version information
func GetVersion() string {
	return fmt.Sprintf("passgo %s (built on %s, commit %s, %s/%s)",
		Version, BuildTime, GitCommit, runtime.GOOS, runtime.GOARCH)
}
