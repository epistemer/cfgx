// Package mailer sends mail. It depends only on the settings.Mailer
// interface, so any command whose configuration has those keys can build
// one, and no other can.
package mailer

import (
	"fmt"
	"net"
	"strconv"

	"zp/internal/settings"
)

// Mailer sends mail through an SMTP server.
type Mailer struct{ cfg settings.Mailer }

// New returns a mailer for cfg.
func New(cfg settings.Mailer) *Mailer { return &Mailer{cfg: cfg} }

// Describe says where mail goes, without sending any.
func (m *Mailer) Describe(to string) string {
	addr := net.JoinHostPort(m.cfg.SMTPHost(), strconv.Itoa(m.cfg.SMTPPort()))
	return fmt.Sprintf("from %s to %s via %s (auth %s, tls %t)", m.cfg.AuthEmailFrom(), to, addr, m.cfg.SMTPAuth(), m.cfg.SMTPTLS())
}
