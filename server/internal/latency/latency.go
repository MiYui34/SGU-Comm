// Package latency answers the web app's round-trip probe.
//
// The delay the web app shows is the time a request takes to go to this
// server and come back, measured by the browser itself. This handler is the
// smallest thing that can come back: no body, no work, no database. Anything
// slower would show up as delay the users do not have.
package latency

import "net/http"

// Handler answers GET and HEAD with no content.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		// A cached answer would look like a fast network.
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	})
}
