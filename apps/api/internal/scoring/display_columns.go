package scoring

import "strings"

// DisplayStatKeys returns ordered raw stat keys used for box-score style tables.
// Keys align with DefaultRules (excluding bonus_* threshold rules).
func DisplayStatKeys(sport string) []string {
	switch strings.ToLower(strings.TrimSpace(sport)) {
	case "nfl":
		return []string{
			"pass_yd", "pass_td", "pass_int", "pass_2pt",
			"rush_yd", "rush_td", "rush_2pt",
			"rec", "rec_yd", "rec_td", "rec_2pt",
			"fum_lost",
			"def_int", "def_fr", "def_sack", "def_td", "def_safe", "def_block_kick",
			"st_td",
			"fgm_0_19", "fgm_20_29", "fgm_30_39", "fgm_40_49", "fgm_50p",
			"fgmiss", "xpm", "xpmiss",
		}
	case "nba":
		return []string{"pts", "reb", "ast", "stl", "blk", "to", "fg3m"}
	case "mlb":
		return []string{
			"hit", "run", "rbi", "hr", "sb", "bb", "k",
			"win", "loss", "qs", "sv", "ip", "er", "k_p",
		}
	default:
		return nil
	}
}
