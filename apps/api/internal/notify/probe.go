package notify

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/bulbousoars/lunarleague/apps/api/internal/config"
)

// LogSMTPReachability logs a warning if the SMTP TCP endpoint cannot be reached.
// It does not send mail or validate TLS/auth — only catches DNS/firewall/wrong host mistakes early.
func LogSMTPReachability(ctx context.Context, cfg config.SMTP) {
	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		return
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", cfg.Port))
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		slog.Warn("smtp: TCP dial failed — magic links and league emails will fail until SMTP is reachable",
			"addr", addr, "err", err)
		return
	}
	_ = c.Close()
	slog.Info("smtp: relay TCP reachable", "addr", addr)
}
