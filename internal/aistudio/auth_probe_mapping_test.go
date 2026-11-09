package aistudio

import (
	"strings"
	"testing"
	"time"
)

// These are format regressions, not substitutes for the real Google device test.
func TestAuthProbeHostOnlyMapping(t *testing.T) {
	state, err := parseCookieJSON(`[{"name":"synthetic","value":"test-only","domain":"accounts.google.com","path":"/","hostOnly":true,"session":true,"sameSite":"unspecified"}]`)
	if err != nil {
		t.Fatal("synthetic parse failed")
	}
	c := state.Cookies[0]
	if !c.HostOnly || c.Domain != "accounts.google.com" || c.Expires != -1 || c.SameSite != "" {
		t.Fatal("host-only or session semantics changed")
	}
	exact, _ := state.CookieHeader("https://accounts.google.com/", time.Now())
	sub, _ := state.CookieHeader("https://child.accounts.google.com/", time.Now())
	if exact == "" || sub != "" {
		t.Fatal("host-only scope violated")
	}
	state.Cookies[0].HostOnly = false
	state.Cookies[0].Domain = ".google.com"
	domain, _ := state.CookieHeader("https://aistudio.google.com/", time.Now())
	if !strings.Contains(domain, "synthetic=") {
		t.Fatal("domain cookie lost")
	}
	if cookieSameSite("no_restriction") != "None" || cookieSameSite("unspecified") != "" {
		t.Fatal("sameSite conversion changed")
	}
}
