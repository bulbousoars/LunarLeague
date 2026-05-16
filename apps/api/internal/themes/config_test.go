package themes

import "testing"

func TestDefaultConfigHas30Themes(t *testing.T) {
	c := DefaultConfig()
	if len(c) != len(Catalog) {
		t.Fatalf("got %d entries want %d", len(c), len(Catalog))
	}
	if c["franchise_stack_win"].Multiplier != 1.06 {
		t.Fatalf("franchise multiplier")
	}
}

func TestMergePatchRejectsUnavailable(t *testing.T) {
	c := DefaultConfig()
	err := c.MergePatch(Config{
		"weather_goblin": {Enabled: true},
	})
	if err == nil {
		t.Fatal("expected error enabling weather_goblin")
	}
}

func TestMergePatchEnableFranchise(t *testing.T) {
	c := DefaultConfig()
	if err := c.MergePatch(Config{
		"franchise_stack_win": {Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}
	if !c["franchise_stack_win"].Enabled {
		t.Fatal("expected enabled")
	}
}
