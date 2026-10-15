package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/pem"
	"log/slog"
	"time"

	"kallisti.chat/server/internal/store"
)

// Temporary (guest) accounts live this long after their last visit.
const guestLifetime = 24 * time.Hour

// cleanGuests deletes temporary accounts not seen for guestLifetime, with
// their Mumble registration (the account stays if that fails, for the next
// round). Online ones count as seen.
func cleanGuests(db *store.Store, now time.Time, online func(name string) bool, unregister func(certHash string) error) {
	idle, err := db.IdleTemporaryAccounts(now.Add(-guestLifetime))
	if err != nil {
		slog.Error("finding idle guest accounts", "err", err)
		return
	}
	for _, acc := range idle {
		if online(acc.Name) {
			db.TouchAccount(acc.ID, now)
			continue
		}
		if err := unregister(accountCertHash(acc)); err != nil {
			slog.Error("removing a guest's registration", "account", acc.Name, "err", err)
			continue
		}
		if err := db.DeleteAccount(acc.ID); err != nil {
			slog.Error("deleting a guest account", "account", acc.Name, "err", err)
		}
	}
}

// accountCertHash is the Mumble certificate hash of an account (SHA-1 of
// the DER certificate, as Mumble's).
func accountCertHash(acc store.Account) string {
	block, _ := pem.Decode(acc.CertPEM)
	if block == nil {
		return ""
	}
	return certHash(block.Bytes)
}

func certHash(der []byte) string {
	sum := sha1.Sum(der)
	return hex.EncodeToString(sum[:])
}
