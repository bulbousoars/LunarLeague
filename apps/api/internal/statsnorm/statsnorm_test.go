package statsnorm

import (
	"reflect"
	"testing"
)

func TestNormalizeStatMap_NFLPassIntAlias(t *testing.T) {
	in := map[string]float64{"int": 2, "pass_yd": 220.0}
	out := NormalizeStatMap("nfl", "sleeper", in)
	want := map[string]float64{"pass_int": 2, "pass_yd": 220}
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("got %#v want %#v", out, want)
	}
}

func TestNormalizeStatMap_PassthroughNBA(t *testing.T) {
	in := map[string]float64{"pts": 21, "reb": 9}
	out := NormalizeStatMap("nba", "sleeper", in)
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("got %#v want %#v", out, in)
	}
}
