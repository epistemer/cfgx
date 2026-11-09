// Package db opens zp's database. It takes the settings.DB interface,
// whose computed DSN builds the connection string.
package db

import "zp/internal/settings"

// Target is where Open would connect, with the password redacted.
func Target(cfg settings.DB) string {
	return cfg.DBUser() + "@" + cfg.DBHost() + "/" + cfg.DBName() + "?sslmode=" + cfg.DBSslmode()
}

// DSN is the connection string Open would use.
func DSN(cfg settings.DB) string { return cfg.DSN() }
