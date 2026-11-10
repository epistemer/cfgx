package settings

import (
	"time"

	mrand "math/rand/v2"
)

var rand = 1

func (g *dbGroup) DSN() string { return "postgres://" + g.DBHost() }

func (g *dbGroup) Rand() *mrand.Rand { return nil }

func (g *schedulerGroup) RetryPolicy() (time.Duration, error) { return time.Second, nil }

func (g *schedulerGroup) Named() (n int, err error) { return }

func (g *schedulerGroup) Takes(x int) int { return x }

func (g *schedulerRetryGroup) Backoff() time.Duration { return time.Second }
