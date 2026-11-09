// Package types holds types computed settings return.
package types

import "time"

// RetryPolicy is how the scheduler retries a failed task.
type RetryPolicy struct {
	Max   int
	Delay time.Duration
}
