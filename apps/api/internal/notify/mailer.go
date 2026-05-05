// Package notify owns email + push delivery and the per-user notification feed.
package notify

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/bulbousoars/lunarleague/apps/api/internal/config"
	mail "github.com/wneessen/go-mail"
)

// Mailer is the abstraction handlers + workers depend on. SMTPMailer is the
// only implementation today; LogMailer is useful in tests.
type Mailer interface {
	SendMagicLink(ctx context.Context, to, link string) error
	SendDigest(ctx context.Context, to, subject, html, text string) error
	// SendLeagueCreated emails the commissioner after POST /v1/leagues (setup + invite URLs).
	SendLeagueCreated(ctx context.Context, to, leagueName, setupURL, inviteShareURL string) error
}

type smtpMailer struct {
	cfg config.SMTP
}

func NewSMTPMailer(cfg config.SMTP) Mailer {
	return &smtpMailer{cfg: cfg}
}

func (m *smtpMailer) SendMagicLink(ctx context.Context, to, link string) error {
	subject := "Your Lunar League sign-in link"
	text := fmt.Sprintf(
		"Click to sign in to Lunar League:\n\n%s\n\nThe link expires in 15 minutes. If you didn't request this, ignore it.\n",
		link)
	html := fmt.Sprintf(`
<!doctype html>
<html><body style="font-family:system-ui,sans-serif;max-width:560px;margin:24px auto;color:#1f2937">
  <h2 style="margin:0 0 12px 0">Sign in to Lunar League</h2>
  <p>Click the button below to sign in. The link expires in 15 minutes.</p>
  <p><a href="%s" style="display:inline-block;padding:10px 16px;background:#111827;color:white;text-decoration:none;border-radius:6px">Sign in</a></p>
  <p style="font-size:12px;color:#6b7280">If you didn't request this, ignore this email.</p>
</body></html>`, link)
	return m.send(ctx, to, subject, text, html)
}

func (m *smtpMailer) SendDigest(ctx context.Context, to, subject, html, text string) error {
	return m.send(ctx, to, subject, text, html)
}

func (m *smtpMailer) SendLeagueCreated(ctx context.Context, to, leagueName, setupURL, inviteShareURL string) error {
	nameOneLine := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(leagueName, "\r", " "), "\n", " "))
	subject := fmt.Sprintf("Your league %s is ready — Lunar League", nameOneLine)
	nameEsc := html.EscapeString(nameOneLine)
	setupEsc := html.EscapeString(setupURL)
	inviteEsc := html.EscapeString(inviteShareURL)
	text := fmt.Sprintf(
		"You created \"%s\" on Lunar League.\n\nFinish setup:\n%s\n\nShare this invite link with managers:\n%s\n",
		nameOneLine, setupURL, inviteShareURL)
	htmlBody := fmt.Sprintf(`
<!doctype html>
<html><body style="font-family:system-ui,sans-serif;max-width:560px;margin:24px auto;color:#1f2937">
  <h2 style="margin:0 0 12px 0">League created</h2>
  <p><strong>%s</strong> is ready. Finish commissioner setup and invite your league.</p>
  <p><a href="%s" style="display:inline-block;padding:10px 16px;background:#111827;color:white;text-decoration:none;border-radius:6px">Open setup</a></p>
  <p style="margin-top:20px;font-size:14px">Invite link (share with friends):</p>
  <p style="word-break:break-all;font-size:13px"><a href="%s">%s</a></p>
  <p style="font-size:12px;color:#6b7280">You received this because you created this league in Lunar League.</p>
</body></html>`, nameEsc, setupEsc, inviteEsc, inviteEsc)
	return m.send(ctx, to, subject, text, htmlBody)
}

func (m *smtpMailer) send(ctx context.Context, to, subject, text, html string) error {
	msg := mail.NewMsg()
	if err := msg.From(m.cfg.From); err != nil {
		return fmt.Errorf("from: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("to: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, text)
	if html != "" {
		msg.AddAlternativeString(mail.TypeTextHTML, html)
	}

	opts := []mail.Option{mail.WithPort(m.cfg.Port)}
	if m.cfg.Username != "" {
		opts = append(opts, mail.WithUsername(m.cfg.Username), mail.WithPassword(m.cfg.Password))
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthPlain))
	}
	if m.cfg.TLS {
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	} else {
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	}
	c, err := mail.NewClient(m.cfg.Host, opts...)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	return c.DialAndSendWithContext(ctx, msg)
}
