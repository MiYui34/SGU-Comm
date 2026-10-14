package password

import "testing"

func TestHash(t *testing.T) {
	h, err := Hash("secret password")
	if err != nil {
		t.Fatal(err)
	}
	if !Check(h, "secret password") || Check(h, "other password") {
		t.Fatal("check failed")
	}
	h2, _ := Hash("secret password")
	if h == h2 {
		t.Fatal("no salt")
	}
}
