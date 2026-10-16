# Review efficiency, DRY, architecture and security

## Summary

Review of repository revision `52719d0`. No application code was changed. Findings below are prioritized by impact; a severity does not imply an unauthenticated exploit unless stated.

**Independent follow-up:** the [6.1-sol report](repository-review-6-1-sol.md) confirms these nine defects, adds four findings, and reclassifies finding 2 below as **Medium capacity/reliability**, rather than High. This initial report is otherwise preserved as the original audit record.

## Requirements

- Inspect the Go server, web client, protocol package, desktop bridge and deployment boundaries.
- Prioritize concrete findings with file/line references, impact and focused recommendations.
- Run relevant existing checks where feasible and state review limitations.
- Do not implement fixes as part of this audit.

## Acceptance Criteria

- [x] Findings validated against source and reported with severity and references.
- [x] Existing checks and targeted reproductions run; limitations recorded.

## Findings

### 1. High: pre-authentication bridge buffering has no cumulative limit

- **References:** `server/internal/bridge/bridge.go:157-159,372-395`.
- `readHello` appends every binary frame to `early` until Authenticate arrives. The 8 MiB WebSocket message limit bounds individual messages, not their total. The ten-second deadline and connection-count limits do not bound memory consumed within that window.
- This path applies to all Murmur-mode guests and to authenticated web accounts in either deployment mode. An attacker can send valid non-Authenticate frames repeatedly and exhaust memory before the greeting timeout.
- **Reproduced:** nine approximately 1 MiB frames followed by Authenticate produced a retained greeting of 9,437,251 bytes, exceeding the 8,388,608-byte per-message limit. No remote/public service was used.
- **Recommendation:** impose a small cumulative greeting budget before appending; reject excess data. Prefer admitting only the expected greeting message types.

### 2. High availability impact: bootstrap fills its own outbound queue and disconnects new clients

- **References:** `server/internal/mumbled/session.go:289-305,326-402`; `server/internal/mumbled/server.go:63,275-284`.
- `join` queues the entire initial channel/user state before starting the writer. The queue holds 512 frames; overflowing a control frame closes the connection. Normal server state can therefore look like a slow reader before the writer has even started.
- **Reproduced:** 508 total channels, no existing users or links, filled the queue and closed the new connection. More users or channel links reduce the channel threshold.
- **Recommendation:** distinguish ordered bootstrap delivery from steady-state backpressure. Start an appropriately ordered bootstrap writer, or deliver a snapshot through a bounded streaming mechanism; do not merely increase the fixed limit.

### 3. Medium security: online unregistration collapses distinct web identities to one author ID

- **References:** `server/internal/mumbled/perms.go:445-472`; `server/internal/mumble/sniffer.go:88-95`; `server/internal/sidecar/hub.go:548`; `server/internal/sidecar/edit.go:40-47`.
- The server sends `UserId = 0xffffffff` when an online user is unregistered. The identity sniffer interprets every present UserId as a valid registration, retaining `Registered=true` with ID 4,294,967,295.
- If two web users are unregistered while online, their subsequent sidecar posts are attributed to that same ID. Message ownership compares IDs, so either can edit/delete messages subsequently posted by the other. Reactions and read marks also share the false identity. Existing messages bearing their original IDs are not made mutually editable by this bug.
- **Reproduced:** real Sniffer frame processing followed by Hub requests assigned Alice and Bob the same sentinel ID; Bob successfully edited Alice's newly posted message. This reproduction models the documented server unregistration frames; it is not a full browser/admin integration test.
- **Recommendation:** normalize the unregistered sentinel to `Registered=false` at the protocol boundary. Add registration -> unregistration -> sidecar ownership tests.

### 4. Medium security/resource accounting: parallel uploads bypass the daily quota

- **References:** `server/internal/files/files.go:91-128,254-267`.
- Both HTTP uploads and native-image saves check the current byte count and record a new upload in separate operations. Concurrent uploads can all observe the same remaining allowance. Serializing individual SQLite statements does not make this read/check/write operation atomic.
- **Reproduced:** two synchronized six-byte HTTP uploads both returned 200 and stored twelve bytes against a ten-byte quota.
- **Recommendation:** atomically reserve capacity per author, release failed reservations, and use the same accounting primitive for HTTP and native uploads. Keep slow body reads outside a global database transaction.

### 5. Medium efficiency: reaction reads perform quadratic ordering work

- **References:** `server/internal/store/reactions.go:62-74`; `server/internal/store/store.go:82-89,707-743,153-155`.
- Each returned reaction row orders itself through a correlated `min(created_at)` subquery over its whole message/emoji group. The primary key locates that group but does not index its creation time. A group with N reacting users incurs approximately N squared scanning work.
- The query runs for history and `MessageByID`, including reaction mutations. The store's single SQL connection means expensive reads delay unrelated database work too. Counts can accumulate over the message lifetime, not just among currently online users.
- **Measured locally with the actual Go store/SQLite driver:** mean `MessageByID` for 250/500/1,000/2,000 users on one emoji was approximately 13/50/199/802 ms (three reads each; synthetic microbenchmark, not a production load test).
- **Recommendation:** compute the earliest timestamp once per `(message_id, emoji)` group and join it to the result, or use an appropriate indexed ordering strategy. Add a scaling benchmark.

### 6. Medium efficiency: stored-chat state and rendered history are unbounded

- **References:** `apps/web/src/lib/textChannels.ts:268-269,302-318`; `apps/web/src/components/Client.svelte:450-451`; `apps/web/src/components/ChatPane.svelte:464-504`.
- Every live post and loaded history page is retained. Each new message rebuilds a Map and sorts the whole channel array, while the view maps/groups all retained messages and renders the loaded list without windowing. In contrast, the raw Mumble chat has a 1,000-message retention cap.
- **Reproduced:** 5,000 live events retained all 5,000 messages. The full series took approximately 718 ms in a reducer-only local Bun run, excluding DOM/layout work. Existing display-object caching is useful, but does not bound array processing or DOM size.
- **Recommendation:** bound retained history and previews, window/virtualize rendered messages, and add an append fast path for increasing live IDs. Preserve explicit backward pagination.

### 7. Medium correctness: message IDs are reused despite monotonic read cursors

- **References:** `server/internal/store/store.go:108-117,623-632,760-782,847-852,885-897`.
- `messages.id` uses SQLite `INTEGER PRIMARY KEY`, not a non-reusing sequence. Deleting the table-wide highest message ID allows the next insert to reuse it. Read marks only increase, and unread queries use `id > read_mark`.
- **Reproduced:** mark message 1 read, delete it, post a new mention; the new message received ID 1 and the unread-mention query returned no message. This requires deletion of the table-wide maximum, not merely a channel-local maximum.
- **Recommendation:** migrate to non-reused monotonic message IDs and add a delete/reinsert/read-mark regression test.

### 8. Medium resource control: voice-target updates bypass the control-message limiter

- **References:** `server/internal/mumbled/messages.go:132-163,799-814`; `server/internal/mumbled/targets.go:49-73`.
- `TypeVoiceTarget` does not consult the limiter used for other control mutations. Repeated frames can force protobuf parsing, allocation, target cleaning and shared-state lock acquisition. Capping retained target counts does not bound request frequency.
- **Reproduced:** with an exhausted, non-refilling client limiter, a VoiceTarget update was still applied.
- **Recommendation:** give target/control updates an explicit suitable rate budget, checked before expensive processing. Preserve a separate allowance for actual voice packets.

### 9. Medium reliability: the desktop upstream TLS handshake can hang indefinitely

- **References:** `apps/desktop/src-tauri/src/bridge.rs:317-329`.
- TCP connect has a ten-second timeout, but `tls.connect` does not. A server that accepts TCP and stalls TLS keeps the bridge connection task alive, including after the renderer disconnects, because forwarding/close handling has not started yet.
- **Recommendation:** bound the complete upstream connection/handshake operation and cancel it when its client disappears. Source-validated; no stalled-TLS runtime test added.

## Architecture and DRY recommendations

These are focused follow-ups, not reasons for a rewrite:

- **Separate reliable lifecycle delivery from lossy traffic.** `server/internal/mumbled/mirrors.go:182-205` drops queued callbacks on overflow. `server/internal/mumbled/server.go:230-243` stops the event worker before sessions finish leaving. Consumers rely on ordered joins/moves/leaves to maintain visits and read marks (`server/internal/sidecar/mirrors.go:92-139`). Drain lifecycle work at shutdown and provide reconciliation or explicit reliability semantics on overflow. Source-validated, not load-tested.
- **Marshal broadcasts once.** `server/internal/sidecar/hub.go:669-695` serializes identical events once per recipient. Factor encoding from enqueueing and share immutable encoded bytes across recipients while preserving individual lag/resync handling. This is a clear allocation/DRY improvement, but its production importance needs profiling.
- **Remove repeated whole-user sorting in the sidebar.** Each channel calls `usersIn`, which sorts all users before filtering (`packages/protocol/src/selectors.ts:37-41`; `apps/web/src/components/ChannelSidebar.svelte:68-69`). Group a once-sorted user list per state update instead.
- **Retain the current package boundaries and pure-state approach.** The store/transport/state separation, functional web helpers, narrow deployment adapters and extensive tests are useful foundations. Prioritize explicit invariants (identity, cursor monotonicity, budgets, lifecycle ordering) over introducing a broad service/repository abstraction layer.

## Lower-priority observations

- The Murmur compose deployment opts out of upstream certificate verification by default (`deploy/compose.murmur.yml:14-17`). This is documented for self-signed servers, but on an untrusted network it permits upstream impersonation. Prefer verified TLS or an explicit pin/CA trust mechanism, with insecure operation an intentional opt-out.
- Hashed channel-description updates can retain stale text because description fetching tracks channel IDs rather than `(channel, hash)` (`packages/protocol/src/state.ts:168-183`; `packages/protocol/src/selectors.ts:67-71`; `apps/web/src/lib/connection.svelte.ts:80-87`). Reuse the explicit invalidation lifecycle already present for avatar hashes.
- A conditional ACL TOCTOU exists when permission is granted through a dynamic group such as `@in`: a user can be moved out during the database lookup, yet the resumed ACL write only rechecks session existence (`server/internal/mumbled/perms.go:146-183`). Recheck authorization after reacquiring the state lock. Disconnection itself is already handled; ordinary concurrent ACL edits are serialized by `saveMu`.

## Positive security controls

- Cookie-based sessions use HttpOnly/SameSite and hashed token storage; account requests enforce JSON and browser-origin policy.
- Upload tokens are single-use. Uploads have per-file and time limits, random keys, restrictive disk modes, content sniffing, download disposition for non-images, `nosniff`, and a sandbox CSP.
- Preview fetching validates destination addresses at dial time, including redirects, and bounds size/time/concurrency/cache memory.
- Built web and desktop CSPs block external image/style/font fetches. The sanitizer retaining inline styles is therefore not reported as an external-tracking vulnerability in default deployments.
- Desktop certificate trust is checked through CA validity or explicit certificate offers/pins, and UDP crypto tests include replay/tamper/countermeasure coverage. This was not a cryptographic audit.

## Verification

- `pnpm test`: passed (121 protocol tests, 244 web tests, Go suite).
- `pnpm typecheck`: passed, including Go vet; Svelte reported zero errors/warnings.
- `pnpm --filter @kallisti/web build`: passed.
- `cd server && go test -race -count=1 ./...`: passed, uncached.
- Desktop `cargo test --locked`: passed (31 unit tests, 11 integration tests reported; one ignored unit test). Real-Murmur integration bodies are environment-gated and were not enabled.
- `pnpm audit --prod --json`: zero advisories reported for the production JavaScript dependency graph at review time.
- Targeted Go tests were supplied through a temporary `go test -overlay` outside the checkout, covering greeting accumulation, quota concurrency, bootstrap overflow, voice-target rate limiting, ID reuse, reaction scaling and Sniffer-to-Hub ownership collision. These reproduce current defects, not fixed behavior; no test files were added to the repository.

## Limitations and disposition

- Source review and local targeted tests, not a penetration test, browser QA, fuzz campaign, or production load test. No live deployment or private `.env` was accessed.
- The official Mumble client harness, real-Murmur gated tests and browser-specific audio checks were not enabled. No Go advisory scan was run. The Rust advisory command could not run because the installed mise shim has no configured cargo-audit version.
- No implementation/TDD cycle was appropriate for a review-only request. Existing tests and focused reproductions were used instead.
- The audit is complete. All application findings remain unfixed; this issue records the review, not completion of remediation.
