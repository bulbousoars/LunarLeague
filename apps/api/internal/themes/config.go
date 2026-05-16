package themes

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Entry is one modifier's stored config.
type Entry struct {
	Enabled    bool    `json:"enabled"`
	Strength   float64 `json:"strength,omitempty"`
	Multiplier float64 `json:"multiplier,omitempty"`
	MinStarters int    `json:"min_starters,omitempty"`
}

// Config is the league_settings.theme_modifiers document.
type Config map[string]Entry

// DefaultConfig returns all themes disabled with default tuning fields.
func DefaultConfig() Config {
	out := Config{}
	for _, d := range Catalog {
		e := Entry{Enabled: false, Strength: 1.0}
		switch d.Slug {
		case "franchise_stack_win":
			e.Multiplier = 1.06
			e.MinStarters = 3
		}
		out[d.Slug] = e
	}
	return out
}

// ParseConfig unmarshals JSONB; missing keys are filled from DefaultConfig.
func ParseConfig(raw []byte) (Config, error) {
	base := DefaultConfig()
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return base, nil
	}
	var patch Config
	if err := json.Unmarshal(raw, &patch); err != nil {
		return nil, err
	}
	slugs := SlugSet()
	for slug, e := range patch {
		if _, ok := slugs[slug]; !ok {
			return nil, fmt.Errorf("unknown theme slug %q", slug)
		}
		if err := validateEntry(slug, e); err != nil {
			return nil, err
		}
		base[slug] = e
	}
	return base, nil
}

// MergePatch applies a partial update from the client (commissioner UI).
func (c Config) MergePatch(patch Config) error {
	slugs := SlugSet()
	for slug, e := range patch {
		if _, ok := slugs[slug]; !ok {
			return fmt.Errorf("unknown theme slug %q", slug)
		}
		if err := validateEntry(slug, e); err != nil {
			return err
		}
		cur := c[slug]
		if e.Enabled != cur.Enabled || patchSetsEnabled(slug, e) {
			// allow explicit enabled toggle
		}
		if e.Strength != 0 {
			cur.Strength = e.Strength
		}
		if e.Multiplier != 0 {
			cur.Multiplier = e.Multiplier
		}
		if e.MinStarters != 0 {
			cur.MinStarters = e.MinStarters
		}
		// Enabled is always taken from patch when key present
		cur.Enabled = e.Enabled
		if !isAvailable(slug) && cur.Enabled {
			return fmt.Errorf("theme %q is not available yet", slug)
		}
		c[slug] = cur
	}
	return nil
}

func patchSetsEnabled(slug string, e Entry) bool { return e.Enabled }

func isAvailable(slug string) bool {
	for _, d := range Catalog {
		if d.Slug == slug {
			return d.Available
		}
	}
	return false
}

func validateEntry(slug string, e Entry) error {
	if !isAvailable(slug) && e.Enabled {
		return fmt.Errorf("theme %q is not available", slug)
	}
	if e.Strength != 0 && (e.Strength < 0.25 || e.Strength > 3) {
		return errors.New("strength must be 0.25..3")
	}
	if e.Multiplier != 0 && (e.Multiplier < 0.5 || e.Multiplier > 2) {
		return errors.New("multiplier must be 0.5..2")
	}
	if e.MinStarters != 0 && (e.MinStarters < 2 || e.MinStarters > 9) {
		return errors.New("min_starters must be 2..9")
	}
	return nil
}

// EnabledSlugs returns slugs with enabled=true.
func (c Config) EnabledSlugs() []string {
	var out []string
	for _, d := range Catalog {
		if e, ok := c[d.Slug]; ok && e.Enabled {
			out = append(out, d.Slug)
		}
	}
	return out
}
