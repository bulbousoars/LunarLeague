package sportsdataio

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// VerifyAccess calls a tiny SportsData.io endpoint per league to see which
// sports the API key can access (HTTP 200). Invalid or missing keys return
// verified=false and empty sports.
func VerifyAccess(ctx context.Context, apiKey string) (sports []string, verified bool) {
	return verifyAccessImpl(ctx, apiKey)
}

var verifyCache struct {
	mu       sync.Mutex
	key      string
	until    time.Time
	sports   []string
	verified bool
}

// VerifyAccessCached is like VerifyAccess but caches a positive or negative
// result for a few minutes per key so dashboards do not hammer SportsData.io.
func VerifyAccessCached(ctx context.Context, apiKey string) (sports []string, verified bool) {
	k := strings.TrimSpace(apiKey)
	if k == "" {
		return nil, false
	}
	verifyCache.mu.Lock()
	defer verifyCache.mu.Unlock()
	if k == verifyCache.key && time.Now().Before(verifyCache.until) {
		return append([]string(nil), verifyCache.sports...), verifyCache.verified
	}
	sports, verified = verifyAccessImpl(ctx, apiKey)
	verifyCache.key = k
	verifyCache.until = time.Now().Add(5 * time.Minute)
	verifyCache.sports = append([]string(nil), sports...)
	verifyCache.verified = verified
	return sports, verified
}

func verifyAccessImpl(ctx context.Context, apiKey string) (sports []string, verified bool) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return nil, false
	}
	p := New(key)
	client := &http.Client{Timeout: 8 * time.Second}

	for _, lg := range []string{"nfl", "nba", "mlb"} {
		u, err := p.withKey(host + "/" + lg + "/scores/json/CurrentSeason")
		if err != nil {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "LunarLeague/0.1 (+https://github.com/bulbousoars/LunarLeague)")
		res, err := client.Do(req)
		if err != nil {
			continue
		}
		func() {
			defer res.Body.Close()
			_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64*1024))
		}()
		if res.StatusCode == http.StatusOK {
			sports = append(sports, lg)
		}
	}
	return sports, len(sports) > 0
}
