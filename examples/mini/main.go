// Command mini is a small example application demonstrating cfgx with cobra.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/epistemer/cfgx/examples/mini/internal/settings"
)

func main() {
	root := &cobra.Command{
		Use:   "mini",
		Short: "Minimal cfgx example",
	}
	settings.BindRoot(root)

	root.AddCommand(
		settings.ServeCommand(&cobra.Command{
			Use:   "serve",
			Short: "Run the HTTP server",
		}, runServe),
		settings.MigrateCommand(&cobra.Command{
			Use:   "migrate",
			Short: "Run database migrations",
		}, runMigrate),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServe(_ context.Context, s *settings.Serve) error {
	fmt.Printf("serve: host=%q port=%d tls=%t %s verbose=%t\n",
		s.ServerHost(), s.ServerPort(), s.ServerTLS(), formatDB(s), s.LogVerbose())
	return nil
}

func runMigrate(_ context.Context, m *settings.Migrate) error {
	fmt.Printf("migrate: %s verbose=%t\n", formatDB(m), m.LogVerbose())
	return nil
}

func formatDB(db settings.Database) string {
	return fmt.Sprintf("db_url=%q db_timeout=%d db_debug=%t",
		db.DatabaseURL(), db.DatabaseTimeout(), db.DatabaseDebug())
}
