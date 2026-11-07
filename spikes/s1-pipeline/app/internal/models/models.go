// Package models holds an enum type the schema refers to, and pulls in a
// third-party dependency so that vendor mode has something to vendor.
package models

import "github.com/rogpeppe/go-internal/semver"

// Mode is an enum type, like zp's ZpTaskExecutionMode.
type Mode string

// The Mode values.
const (
	Sequential Mode = "sequential"
	Parallel   Mode = "parallel"
)

// MinVersion exists only to use the third-party dependency.
var MinVersion = semver.Canonical("v1.2")
