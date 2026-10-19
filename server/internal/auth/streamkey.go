package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"kallisti.chat/server/internal/store"
)

// streamKey: GET tells whether the account has a stream key (for OBS, see
// the whip package), POST makes a new one and shows it this once, DELETE
// removes it. Keys are stored hashed, like sessions. Guest accounts get
// none.
func (a *Service) streamKey(w http.ResponseWriter, r *http.Request) {
	acc, ok := a.AccountFor(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not logged in"})
		return
	}
	if !a.StreamKeys {
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, map[string]bool{"enabled": false})
		} else {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "streaming from OBS is off on this server"})
		}
		return
	}
	if acc.Temporary {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "set a password first: guest accounts cannot stream from OBS"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		has, err := a.Store.HasStreamKey(acc.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot read the stream key"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"enabled": true, "exists": has})
	case http.MethodPost:
		if !a.allow(r) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, try again later"})
			return
		}
		b := make([]byte, 32)
		rand.Read(b)
		key := hex.EncodeToString(b)
		if a.Store.SetStreamKey(acc.ID, tokenHash(key), a.Now()) != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot make a stream key"})
			return
		}
		a.streamKeyChanged(acc.Name)
		writeJSON(w, http.StatusOK, map[string]string{"key": key})
	case http.MethodDelete:
		if a.Store.DeleteStreamKey(acc.ID) != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot remove the stream key"})
			return
		}
		a.streamKeyChanged(acc.Name)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (a *Service) streamKeyChanged(name string) {
	if a.StreamKeyChanged != nil {
		a.StreamKeyChanged(name)
	}
}

// StreamKeyAccount is the account of a stream key (not a guest account).
func (a *Service) StreamKeyAccount(key string) (store.Account, bool) {
	if key == "" {
		return store.Account{}, false
	}
	acc, err := a.Store.AccountByStreamKey(tokenHash(key))
	return acc, err == nil && !acc.Temporary
}
