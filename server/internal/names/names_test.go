package names

import "testing"

func TestKey(t *testing.T) {
	cases := map[string]string{
		"  Bob ": "bob",
		"Émile":  "émile",
		"Émile": "émile", // decomposed é becomes the composed form
		"ÉMILE":  "émile",
	}
	for in, want := range cases {
		if got := Key(in); got != want {
			t.Errorf("Key(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValid(t *testing.T) {
	for _, ok := range []string{"alice", "Émile", "bob_2", "x-y.z", "[clan]name", "a@b"} {
		if msg := Check(ok); msg != "" {
			t.Errorf("Check(%q) = %q, want ok", ok, msg)
		}
	}
	// Murmur's default username rule has no spaces, emoji or most punctuation.
	for _, bad := range []string{"x", "two words", "😀😀", "semi;colon", "superuser", "bad\nname", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		if msg := Check(bad); msg == "" {
			t.Errorf("Check(%q) accepted", bad)
		}
	}
}
