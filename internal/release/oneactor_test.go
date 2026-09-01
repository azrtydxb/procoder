package release

import "testing"

// The two spellings GitHub uses for one bot actor, from the two endpoints
// the credit check talks to, must compare equal — and a bot must not
// compare equal to a person who shares its stem.
func TestOneActorComparesBotSpellings(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"app/github-actions", "github-actions[bot]", true},
		{"github-actions[bot]", "app/github-actions", true},
		{"app/github-actions", "app/github-actions", true},
		{"Acroaticum", "acroaticum", true},
		{"github-actions[bot]", "github-actions2", false},
		// The stem a bot and a person can share is not an actor: the bot
		// never compares equal to the person whose login is its stem.
		{"github-actions[bot]", "github-actions", false},
		{"app/github-actions", "github-actions", false},
		{"github-actions", "app/github-actions", false},
		{"", "github-actions[bot]", false},
	}
	for _, c := range cases {
		if got := oneActor(c.a, c.b); got != c.want {
			t.Errorf("oneActor(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
