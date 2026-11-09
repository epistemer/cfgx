// Package server is zp's HTTP server. It takes the settings.Server
// interface, not a config struct.
package server

import (
	"fmt"
	"net"
	"strconv"

	"zp/internal/settings"
)

// Server serves zp's API.
type Server struct{ cfg settings.Server }

// New returns a server for cfg.
func New(cfg settings.Server) *Server { return &Server{cfg: cfg} }

// Addr is the address the server listens on.
func (s *Server) Addr() string {
	return net.JoinHostPort(s.cfg.ServerHost(), strconv.Itoa(s.cfg.ServerPort()))
}

// Describe sums up how the server would run.
func (s *Server) Describe() string {
	scheme := "http"
	if s.cfg.ServerTLSEnabled() {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s (read timeout %s, max header %d bytes)", scheme, s.Addr(), s.cfg.ServerTimeoutRead(), s.cfg.ServerMaxHeader())
}
