package auth

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// getChallenge asks for a challenge: its token ("" when off) and bits.
func getChallenge(t *testing.T, srvURL string) (string, int) {
	t.Helper()
	resp, err := http.Get(srvURL + "/api/challenge")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var v struct {
		Challenge string `json:"challenge"`
		Bits      int    `json:"bits"`
		Expires   int64  `json:"expires"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&v) != nil {
		t.Fatalf("challenge: %d", resp.StatusCode)
	}
	if v.Bits > 0 && (v.Challenge == "" || v.Expires <= time.Now().UnixMilli()) {
		t.Fatalf("challenge %+v", v)
	}
	return v.Challenge, v.Bits
}

// solve finds a nonce, as the browser does.
func solve(challenge string, bits int) string {
	for n := 0; ; n++ {
		nonce := strconv.Itoa(n)
		if sum := sha256.Sum256([]byte(challenge + nonce)); zeroBits(sum[:]) >= bits {
			return nonce
		}
	}
}

func TestZeroBits(t *testing.T) {
	for _, c := range []struct {
		b    []byte
		want int
	}{{[]byte{0x80}, 0}, {[]byte{0x01}, 7}, {[]byte{0, 0x40}, 9}, {[]byte{0, 0}, 16}} {
		if got := zeroBits(c.b); got != c.want {
			t.Errorf("%x: %d", c.b, got)
		}
	}
}

func TestChallenge(t *testing.T) {
	a, srv := setup(t)
	a.GuestAccounts = true
	level := "off"
	a.Captcha = func() string { return level }
	captchaBits["test"] = 8
	t.Cleanup(func() { delete(captchaBits, "test") })

	// Off: nothing to solve, nothing needed.
	if c, bits := getChallenge(t, srv.URL); c != "" || bits != 0 {
		t.Fatalf("off: %q %d", c, bits)
	}
	if resp, v := post(t, srv, "/api/guest", map[string]string{"name": "early"}, nil); resp.StatusCode != http.StatusCreated {
		t.Fatalf("off: %d %v", resp.StatusCode, v)
	}

	level = "test"
	for _, path := range []string{"/api/accounts", "/api/guest"} {
		a.logins = limiter{} // many attempts here
		body := func(name string, proof ...string) map[string]string {
			b := map[string]string{"name": name, "password": "correct horse"}
			if len(proof) == 2 {
				b["challenge"], b["nonce"] = proof[0], proof[1]
			}
			return b
		}
		refused := func(what string, b map[string]string) {
			t.Helper()
			if resp, v := post(t, srv, path, b, nil); resp.StatusCode != http.StatusForbidden {
				t.Fatalf("%s %s: %d %v", path, what, resp.StatusCode, v)
			}
		}
		refused("no proof", body("bot"))
		c, bits := getChallenge(t, srv.URL)
		if bits != 8 {
			t.Fatalf("bits %d", bits)
		}
		nonce := solve(c, bits)
		wrong := nonce + "x"
		for sum := sha256.Sum256([]byte(c + wrong)); zeroBits(sum[:]) >= bits; sum = sha256.Sum256([]byte(c + wrong)) {
			wrong += "x"
		}
		refused("wrong nonce", body("bot", c, wrong))
		forged := "A" + c[1:]
		if c[0] == 'A' {
			forged = "B" + c[1:]
		}
		refused("forged", body("bot", forged, nonce))
		if resp, v := post(t, srv, path, body("human"+path[5:8], c, nonce), nil); resp.StatusCode != http.StatusCreated {
			t.Fatalf("%s solved: %d %v", path, resp.StatusCode, v)
		}
		refused("reused", body("again"+path[5:8], c, nonce))

		// Expired.
		c, _ = getChallenge(t, srv.URL)
		a.Now = func() time.Time { return time.Now().Add(challengeLifetime + time.Second) }
		refused("expired", body("late", c, solve(c, bits)))
		a.Now = time.Now
	}

	a.logins = limiter{}
	// A harder level refuses easier solutions found before.
	c, nonce := "", ""
	for sum := [32]byte{}; c == "" || zeroBits(sum[:]) >= 12; sum = sha256.Sum256([]byte(c + nonce)) {
		c, _ = getChallenge(t, srv.URL)
		nonce = solve(c, 8)
	}
	captchaBits["test"] = 12
	if resp, _ := post(t, srv, "/api/guest", map[string]string{"name": "weak", "challenge": c, "nonce": nonce}, nil); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("easier solution: %d", resp.StatusCode)
	}

	// Logins need none.
	if resp, _ := post(t, srv, "/api/login", map[string]string{"name": "humanacc", "password": "correct horse"}, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("login: %d", resp.StatusCode)
	}
}
