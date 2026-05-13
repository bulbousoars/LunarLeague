// Package statsnorm maps upstream provider stat keys onto canonical Lunar League
// keys (aligned with scoring.DefaultRules and scoring.DisplayStatKeys). Extra
// keys are preserved so we do not drop provider-specific metrics.
package statsnorm

import (
	"strings"
)

type aliasRule struct {
	canonical string
	alts      []string
}

// NormalizeStatMap returns a new map with aliases merged into canonical keys.
// sport is the sport code (nfl, nba, mlb). provider is the DataProvider name
// (sleeper, mlbstatsapi, sportsdataio); it is reserved for provider-specific rules.
func NormalizeStatMap(sport, provider string, in map[string]float64) map[string]float64 {
	if len(in) == 0 {
		return map[string]float64{}
	}
	sport = strings.ToLower(strings.TrimSpace(sport))
	_ = strings.ToLower(strings.TrimSpace(provider))

	rules := rulesForSport(sport)
	if len(rules) == 0 {
		return cloneAdd(in, nil)
	}

	used := make(map[string]struct{}, len(in))
	out := make(map[string]float64, len(in))

	for _, r := range rules {
		var sum float64
		var hit bool
		for _, k := range r.alts {
			if v, ok := in[k]; ok {
				sum += v
				used[k] = struct{}{}
				hit = true
			}
		}
		if hit {
			out[r.canonical] += sum
		}
	}

	for k, v := range in {
		if _, skip := used[k]; skip {
			continue
		}
		out[k] += v
	}
	return out
}

func rulesForSport(sport string) []aliasRule {
	switch sport {
	case "nfl":
		return []aliasRule{
			// Sleeper / some feeds use "int" for passing interceptions; our rules use pass_int.
			{canonical: "pass_int", alts: []string{"int"}},
		}
	default:
		return nil
	}
}

func cloneAdd(in map[string]float64, out map[string]float64) map[string]float64 {
	if out == nil {
		out = make(map[string]float64, len(in))
	}
	for k, v := range in {
		out[k] += v
	}
	return out
}
