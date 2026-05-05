package auth

import "testing"

func TestSafeRedirectPathAllowsRelativeInviteLinks(t *testing.T) {
	got := safeRedirectPath("/leagues/abc/join?code=INVITE")
	if got != "/leagues/abc/join?code=INVITE" {
		t.Fatalf("safeRedirectPath returned %q", got)
	}
}

func TestSafeRedirectPathRejectsExternalTargets(t *testing.T) {
	cases := []string{
		"https://evil.example/leagues/abc/join?code=INVITE",
		"//evil.example/leagues/abc/join?code=INVITE",
		"javascript:alert(1)",
		"",
	}

	for _, tc := range cases {
		if got := safeRedirectPath(tc); got != "" {
			t.Fatalf("safeRedirectPath(%q) returned %q, want empty", tc, got)
		}
	}
}
