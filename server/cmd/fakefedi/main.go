// Command fakefedi runs a fake fediverse instance (see fedifake) on
// 127.0.0.1, over plain HTTP, to try fediverse logins locally: start the
// server with FEDI_INSECURE_DEV_INSTANCES=1 and log in with
// http://127.0.0.1:PORT as the instance. It approves every login, for any
// user name: development only.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"kallisti.chat/server/internal/fedi/fedifake"
)

func main() {
	port := flag.Int("port", 8099, "port on 127.0.0.1")
	noProfile := flag.Bool("no-profile-scope", false, "act as a server without the profile scope (Mastodon before 4.3)")
	flag.Parse()
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Printf("fake fediverse instance on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, &fedifake.Instance{NoProfileScope: *noProfile}))
}
