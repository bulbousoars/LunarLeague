// Package router wires HTTP routes to domain handlers.
package router

import (
	"net/http"
	"time"

	"github.com/bulbousoars/lunarleague/apps/api/internal/auth"
	"github.com/bulbousoars/lunarleague/apps/api/internal/chat"
	"github.com/bulbousoars/lunarleague/apps/api/internal/config"
	"github.com/bulbousoars/lunarleague/apps/api/internal/db"
	"github.com/bulbousoars/lunarleague/apps/api/internal/draft"
	"github.com/bulbousoars/lunarleague/apps/api/internal/httpx"
	"github.com/bulbousoars/lunarleague/apps/api/internal/league"
	"github.com/bulbousoars/lunarleague/apps/api/internal/matchup"
	"github.com/bulbousoars/lunarleague/apps/api/internal/notify"
	"github.com/bulbousoars/lunarleague/apps/api/internal/player"
	"github.com/bulbousoars/lunarleague/apps/api/internal/provider"
	"github.com/bulbousoars/lunarleague/apps/api/internal/roster"
	"github.com/bulbousoars/lunarleague/apps/api/internal/scoring"
	"github.com/bulbousoars/lunarleague/apps/api/internal/trades"
	"github.com/bulbousoars/lunarleague/apps/api/internal/waivers"
	"github.com/bulbousoars/lunarleague/apps/api/internal/ws"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	Cfg      *config.Config
	DB       *db.DB
	Redis    *redis.Client
	Mailer   notify.Mailer
	Provider provider.DataProvider
	Hub      *ws.Hub
}

func New(d *Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(middleware.Timeout(30 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{d.Cfg.PublicWebURL},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	authSvc := auth.NewService(d.DB, d.Mailer, d.Cfg.PublicWebURL, []byte(d.Cfg.SecretKey))
	leagueSvc := league.NewService(d.DB, d.Mailer, d.Cfg.PublicWebURL)
	playerSvc := player.NewService(d.DB)
	rosterSvc := roster.NewService(d.DB)
	scoringSvc := scoring.NewService(d.DB)
	matchupSvc := matchup.NewService(d.DB)
	draftSvc := draft.NewService(d.DB, d.Hub)
	waiverSvc := waivers.NewService(d.DB)
	tradeSvc := trades.NewService(d.DB)
	chatSvc := chat.NewService(d.DB, d.Hub)
	notifySvc := notify.NewService(d.DB, d.Mailer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	r.Route("/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			authSvc.Mount(r)
		})

		r.Route("/sports", func(r chi.Router) {
			r.Get("/", listSportsHandler(d.DB))
		})

		r.Group(func(r chi.Router) {
			r.Use(authSvc.Middleware)

			r.Get("/me", authSvc.MeHandler)
			r.Patch("/me", authSvc.UpdateMeHandler)

			leagueSvc.Mount(r)
			playerSvc.Mount(r)
			rosterSvc.Mount(r)
			matchupSvc.Mount(r)
			draftSvc.Mount(r)
			waiverSvc.Mount(r)
			tradeSvc.Mount(r)
			scoringSvc.Mount(r)
			chatSvc.Mount(r)
			notifySvc.Mount(r)
		})
	})

	r.Get("/ws/draft/{draftID}", d.Hub.DraftHandler(authSvc))
	r.Get("/ws/league/{leagueID}", d.Hub.LeagueHandler(authSvc))

	return r
}

type sportRow struct {
	ID         int    `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	SeasonType string `json:"season_type"`
}

func listSportsHandler(pool *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(r.Context(),
			`SELECT id, code, name, season_type FROM sports ORDER BY id`)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		defer rows.Close()

		out := []sportRow{}
		for rows.Next() {
			var s sportRow
			if err := rows.Scan(&s.ID, &s.Code, &s.Name, &s.SeasonType); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, err)
				return
			}
			out = append(out, s)
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"sports": out})
	}
}
