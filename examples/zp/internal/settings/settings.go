// Package settings is zp's configuration: zz_settings.go is generated from
// ./schema, and this file holds the computed values.
package settings

// genx runs cfgx gen here, then i18n's labels generator: see genx.yaml and
// the go:generate line in main.go.

import (
	"errors"
	"net"
	"net/url"
	"strconv"

	"zp/internal/types"
)

// DSN is the database connection string.
func (g *dbGroup) DSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(g.DBUser(), g.DBPassword().Reveal()),
		Host:     net.JoinHostPort(g.DBHost(), strconv.Itoa(g.DBPort())),
		Path:     g.DBName(),
		RawQuery: "sslmode=" + g.DBSslmode(),
	}
	return u.String()
}

// RetryPolicy is the scheduler's retry settings as one value.
func (g *schedulerGroup) RetryPolicy() (types.RetryPolicy, error) {
	if g.SchedulerRetryMax() < 0 {
		return types.RetryPolicy{}, errors.New("scheduler.retry.max is negative")
	}
	return types.RetryPolicy{Max: g.SchedulerRetryMax(), Delay: g.SchedulerRetryDelay()}, nil
}
