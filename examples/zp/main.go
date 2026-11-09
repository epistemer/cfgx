// Command zp is the example application from interface.md §3.1: three
// commands, each with only the configuration it uses.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"zp/internal/db"
	"zp/internal/mailer"
	"zp/internal/server"
	"zp/internal/settings"
)

func main() {
	root := &cobra.Command{Use: "zp", Short: "ZaynPay backend", SilenceUsage: true}
	settings.BindRoot(root)

	root.AddCommand(
		settings.ServeCommand(&cobra.Command{Use: "serve", Short: "Run the API server"},
			func(_ context.Context, s *settings.Serve) error {
				policy, err := s.RetryPolicy()
				if err != nil {
					return err
				}
				fmt.Println("serving on", server.New(s).Describe())
				fmt.Println("database", db.Target(s))
				fmt.Println("mail", mailer.New(s).Describe("ops@example.com"))
				fmt.Printf("scheduler %s, %s, %d workers, retry %d every %s\n",
					s.SchedulerLocation(), s.SchedulerExecmode(), s.SchedulerWorkers(), policy.Max, policy.Delay)
				return nil
			}),
		settings.MigrateCommand(&cobra.Command{Use: "migrate", Short: "Apply database migrations"},
			func(_ context.Context, m *settings.Migrate) error {
				dir := "up"
				if m.Down() {
					dir = "down"
				}
				fmt.Printf("migrate %s %d steps on %s\n", dir, m.Steps(), db.Target(m))
				return nil
			}),
		settings.TestEmailCommand(&cobra.Command{Use: "test-email", Short: "Send a test email"},
			func(_ context.Context, t *settings.TestEmail) error {
				fmt.Printf("would send %q to %s <%s>: %s\n", t.Template(), t.ToName(), t.ToAddress(), mailer.New(t).Describe(t.ToAddress()))
				return nil
			}),
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
