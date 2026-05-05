package notify

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
)

// SendDueDigests pulls due rows from digest_outbox and sends them. Called by
// the worker on an hourly tick.
func SendDueDigests(ctx context.Context, pool *db.DB, mailer Mailer, webURL string) error {
	rows, err := pool.Query(ctx, `
		SELECT d.id, u.email, u.display_name, d.kind, d.payload
		FROM digest_outbox d
		JOIN users u ON u.id = d.user_id
		WHERE d.sent_at IS NULL AND d.scheduled_at <= now()
		ORDER BY d.scheduled_at
		LIMIT 200`)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	type item struct {
		id, email, name, kind string
		payload               []byte
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.email, &it.name, &it.kind, &it.payload); err != nil {
			return err
		}
		items = append(items, it)
	}

	for _, it := range items {
		subj, html, text := renderDigest(it.kind, it.name, webURL, it.payload)
		if err := mailer.SendDigest(ctx, it.email, subj, html, text); err != nil {
			slog.Warn("digest send failed", "id", it.id, "err", err)
			continue
		}
		_, _ = pool.Exec(ctx,
			`UPDATE digest_outbox SET sent_at = now() WHERE id = $1`, it.id)
	}
	return nil
}

func renderDigest(kind, name, webURL string, _ []byte) (subject, html, text string) {
	switch kind {
	case "weekly_recap":
		subject = "Your weekly Lunar League recap"
	case "draft_reminder":
		subject = "Your draft is starting soon"
	case "waiver_results":
		subject = "Waiver results are in"
	default:
		subject = "Update from Lunar League"
	}
	text = fmt.Sprintf("Hey %s,\n\nThere's an update for you on Lunar League: %s\n\nVisit %s for details.\n", name, kind, webURL)
	html = fmt.Sprintf(`<p>Hey %s,</p><p>There's an update for you on Lunar League: <strong>%s</strong></p><p><a href="%s">Open Lunar League</a></p>`, name, kind, webURL)
	return
}
