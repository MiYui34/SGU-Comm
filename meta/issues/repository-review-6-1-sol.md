# Independent repository review by 6.1-sol

## Summary

Independent review by `openai/gpt-6.1-sol` of application revision `52719d0`, following the [initial audit](repository-review.md) recorded in commit `a4332d6`. The reviewer inspected the implementation before reading the first report to reduce anchoring. This document consolidates its findings, comparison and subsequent local validation; it is not a verbatim transcript.

**Bottom line:** the first report's nine defects are supported, with bootstrap overflow better classified as **Medium capacity/reliability**, rather than the initial report's High availability classification. The independent review found four additional defects: quadratic HTML stripping under the global server lock, ineffective listener-permission revocation, established sockets surviving session revocation, and microphone tracks leaking after failed worklet initialization. The combined assessment is **two High and eleven Medium findings**. No application fixes were made.

## Requirements

- Use `openai/gpt-6.1-sol` for the independent review.
- Inspect source before reading the first report to reduce anchoring.
- Validate concrete findings and distinguish exploitable defects, conditional risks and optional improvements.
- Produce a separate report, noting material agreements, corrections and additional findings.
- Do not implement application changes.

## Acceptance Criteria

- [x] The requested model completes its review and its findings are validated against source or targeted local tests.
- [x] A separate report and verification limitations are recorded.

## Prioritized findings

Reproduction results for findings carried over from the first report are from that audit; the independent reviewer confirmed those defects against source. The four additional findings were reproduced during the second review and independently rerun locally.

### R1. High: quadratic HTML stripping blocks the global Mumble server lock — additional finding

- **References:** `server/internal/mumbled/messages.go:75-95,230-237,268-276,351-355`; `server/internal/mumbled/blobs.go:58-80`.
- `plainText` repeatedly removes HTML tags until its output stops changing. Removing one tag can expose another, so input such as `strings.Repeat("<", N) + strings.Repeat("b>", N)` requires approximately N passes over shrinking input: quadratic work despite Go's linear-time regular-expression engine.
- A connected guest can submit their own comment. Its stripped result fits the comment limit even when the original input is much larger. Validation and application both strip it while holding `Server.mu`, preventing unrelated routing and state work from acquiring the global lock.
- **Reproduced:** sanitizer timings scaled from tens of milliseconds at 6 KB to seconds at 48 KB. The independent reviewer measured approximately 3.3 seconds of global-lock blocking for a 36 KB guest comment; a subsequent local rerun measured approximately 7.8 seconds. These are variable local microbenchmarks, not production latency estimates, but demonstrate a multi-second denial of service with small input.
- **Recommendation:** use bounded, single-pass stripping that does not reinterpret its output as fresh markup. Normalize once outside the global lock, then authorize and apply the normalized value inside a short critical section. Add adversarial scaling and guest-comment regression tests.

### R2. High: pre-authentication bridge buffering has no cumulative byte limit

- **References:** `server/internal/bridge/bridge.go:157-159,372-395`.
- `readHello` retains every binary frame until Authenticate arrives. The 8 MiB per-message limit and ten-second timeout do not bound the cumulative memory retained during a greeting. This applies to Murmur-mode guests and authenticated web accounts in either deployment mode.
- **Reproduced:** nine approximately 1 MiB frames retained 9,437,251 bytes before authentication completed, exceeding the 8,388,608-byte per-message limit. Repeating this can exhaust memory within the greeting window.
- **Recommendation:** enforce a small cumulative greeting budget before appending, preferably accepting only the expected greeting message types.

### R3. Medium: revoking Listen permission does not stop an existing listener — additional finding

- **References:** `server/internal/mumbled/messages.go:335-343`; `server/internal/mumbled/perms.go:90-114`; `server/internal/mumbled/targets.go:118-130,186-190`.
- Listen permission is checked when a subscription is added. ACL changes invalidate permission caches but do not reconcile existing subscriptions; voice routing trusts the retained Listening entries without checking current Listen permission.
- An existing listener outside the channel therefore continues receiving its audio after both Listen and Enter are denied. This is distinct from the policy question of whether current channel occupants should be evicted when Enter is revoked.
- **Reproduced:** both a direct state test and local TLS clients exercising an actual ACL update continued to deliver audio to the revoked listener. Official Murmur behavior was not compared.
- **Recommendation:** reconcile subscriptions when permissions change and broadcast removals. Preserve a current-authorization invariant for both normal and voice-target listener routing, with revocation tests.

### R4. Medium: established sockets survive logout-all and password session invalidation — additional finding

- **References:** `server/internal/auth/auth.go:316-328,331-377`; `server/internal/bridge/bridge.go:147-152,186-218,330-345`; `server/cmd/server/main.go:131-143`.
- The bridge authenticates the HTTP cookie once and then maintains a certificate-backed upstream connection. Deleting HTTP sessions does not cancel an already established bridge and sidecar connection.
- A holder of an established connection can therefore retain account activity after logout-all or password-change session invalidation. Rejecting the cookie on the next HTTP request does not revoke that existing capability.
- **Reproduced:** a local integration test used the real auth, bridge, sidecar and built-in server components. After logout-all, cookie lookup failed but the existing socket still successfully posted a message.
- **Recommendation:** associate active connections with their session/account and cancel affected sockets on logout, logout-all and password-change invalidation. Do not depend on a cooperative frontend to close them.

### R5. Medium: online unregistration collapses distinct web identities to one author ID

- **References:** `server/internal/mumbled/perms.go:445-472`; `server/internal/mumble/sniffer.go:88-95`; `server/internal/sidecar/hub.go:548`; `server/internal/sidecar/edit.go:40-47`; `packages/protocol/src/state.ts:186-202`.
- An online unregistration sends `UserId = 0xffffffff`. The sniffer treats a present UserId as a valid registration rather than normalizing the sentinel. Two online-unregistered web users subsequently share author ID 4,294,967,295 and can edit/delete each other's subsequent posts. Reactions and read marks also share the false identity; historical posts with the original IDs are not made mutually editable.
- **Reproduced:** actual Sniffer frame processing followed by Hub requests allowed one such user to edit the other's new message. This was not a full browser/admin integration test.
- **Recommendation:** normalize the sentinel at both Go and TypeScript protocol boundaries, and test registration → unregistration → ownership behavior.

### R6. Medium: concurrent uploads bypass the daily quota

- **References:** `server/internal/files/files.go:91-128,254-267`.
- HTTP uploads and native-image saves separately check current usage and record completed uploads. Concurrent requests can all observe the same allowance; serializing SQL statements does not make the compound operation atomic.
- **Reproduced:** two synchronized six-byte HTTP uploads both returned 200 and stored twelve bytes against a ten-byte quota.
- **Recommendation:** use atomic per-author reservations shared by both upload paths, releasing failed reservations. Keep slow body reads outside a global database transaction.

### R7. Medium: bootstrap fills its own outbound queue and disconnects new clients

- **References:** `server/internal/mumbled/session.go:289-305,326-402`; `server/internal/mumbled/server.go:63,275-284`.
- `join` queues the complete initial snapshot before starting its writer. The 512-frame outbox treats the resulting overflow as a slow client and closes the connection, even though no writer has had a chance to drain it.
- **Reproduced:** 508 total channels with no existing users or links closed a new connection. Users and channel links reduce the channel threshold.
- **Severity correction:** this is a meaningful capacity/reliability defect, but depends on accumulated or configured state. Medium is more proportionate than the initial report's High availability label.
- **Recommendation:** separate ordered bootstrap delivery from steady-state backpressure through bounded streaming or appropriately ordered writer startup; merely enlarging the queue postpones the failure.

### R8. Medium: reaction reads perform quadratic ordering work

- **References:** `server/internal/store/reactions.go:62-74`; `server/internal/store/store.go:82-89,153-155,707-743`.
- A correlated `min(created_at)` ordering subquery scans the same message/emoji group for each returned row. The primary key locates the group but does not eliminate repeated scanning. One emoji with N reacting users therefore produces approximately quadratic work.
- **Measured in the initial audit with the actual store/SQLite driver:** mean message reads for 250/500/1,000/2,000 reacting users took approximately 13/50/199/802 ms locally. The single SQL connection amplifies impact on unrelated database operations; reaction counts can accumulate over the message lifetime.
- **Recommendation:** calculate each group's earliest timestamp once and join it, or use an appropriate indexed ordering strategy. Add a scaling benchmark.

### R9. Medium: stored-chat state and rendered history are unbounded

- **References:** `apps/web/src/lib/textChannels.ts:268-269,302-318`; `apps/web/src/components/Client.svelte:450-451`; `apps/web/src/components/ChatPane.svelte:464-504`.
- Live posts and fetched history accumulate indefinitely. Each live post rebuilds a Map and sorts the retained channel array; the view processes and renders the loaded history without windowing. Existing display-object caching helps but does not bound retained state, whole-array processing or DOM size.
- **Reproduced in the initial audit:** 5,000 live events retained all 5,000 messages, taking approximately 718 ms for the reducer series alone in a local Bun run, excluding DOM/layout.
- **Recommendation:** bound retained history/previews, window the rendered list, and fast-path increasing live IDs while preserving explicit backward pagination.

### R10. Medium: message IDs are reused despite monotonic read cursors

- **References:** `server/internal/store/store.go:108-117,623-632,760-782,847-852,885-897`.
- SQLite `INTEGER PRIMARY KEY` without a non-reusing sequence permits reuse after deletion of the table-wide maximum ID. Read marks only increase and unread queries require `id > read_mark`, allowing a new message to look already read.
- **Reproduced:** read message 1, delete it, insert a new mention: the new ID was 1 and unread mentions returned nothing. This requires deletion of the table-wide maximum, not just a channel's maximum.
- **Recommendation:** migrate to non-reused monotonic IDs and add a delete/reinsert/read-mark regression test.

### R11. Medium: voice-target updates bypass the control-message limiter

- **References:** `server/internal/mumbled/messages.go:132-163,799-814`; `server/internal/mumbled/targets.go:49-73`.
- `TypeVoiceTarget` bypasses the control limiter despite doing protobuf parsing, allocation, target cleanup and shared-lock work. A cap on retained targets does not bound update frequency. Practical impact depends on workload and request rate.
- **Reproduced:** an exhausted, non-refilling client limiter still allowed a VoiceTarget mutation.
- **Recommendation:** apply a suitable control/target budget before expensive processing, with enough burst capacity for legitimate target updates. Keep actual voice-packet allowances separate.

### R12. Medium: the desktop upstream TLS handshake can stall indefinitely

- **References:** `apps/desktop/src-tauri/src/bridge.rs:118-122,317-329`.
- TCP connect has a ten-second timeout, but `tls.connect` has none. A peer accepting TCP and stalling TLS can keep the pending task alive after the renderer disconnects, before forwarding/close handling starts.
- **Recommendation:** bound the complete connection/handshake operation and cancel it when the client disappears. Source-validated; no stalled-TLS runtime reproduction was performed.

### R13. Medium: failed capture initialization leaks microphone tracks — additional finding

- **References:** `apps/web/src/lib/voice/capture.svelte.ts:239-276,304-310,339-348`.
- `start` acquires a local `next` MediaStream, then awaits `workletNode()` before assigning it to cleanup-managed `stream`. If initialization rejects, the error is caught but `next` is never stopped. Neither `stop()` nor `close()` can release tracks they never acquired ownership of. A cached rejected initialization promise can make repeated attempts fail similarly.
- **Reproduced:** the actual TS/Svelte capture source, compiled with mocked media APIs and a rejected worklet `addModule`, reported the initialization error but called track `stop()` zero times after start/stop/close. This demonstrates an active-resource cleanup failure, not continued audio transmission, and was not hardware/browser QA.
- **Recommendation:** retain explicit ownership of the pending stream until transfer and stop its tracks on every failure path. Make retry after failed worklet initialization deliberate and tested.

## Architecture, efficiency and DRY

The package boundaries and functional state helpers are useful foundations. Neither review recommends a framework migration or broad rewrite. Prioritize explicit identity, authorization, resource-budget and lifecycle invariants.

1. **Keep expensive work and database waits out of the global voice-state lock.** R1 is an immediate example. Registration also calls `ReservedName` while holding the lock (`server/internal/mumbled/messages.go:712-715`); the production callback queries the single SQL connection (`server/cmd/server/builtin.go:36-41`). Slow reaction reads can therefore propagate from storage contention into voice-server latency. Snapshot/compute outside the lock, then revalidate relevant state before committing.
2. **Separate reliable lifecycle events from lossy traffic.** Mirror callbacks can be dropped on queue overflow (`server/internal/mumbled/mirrors.go:182-205`), and shutdown stops the worker before sessions finish emitting leave events (`server/internal/mumbled/server.go:230-243`). Sidecar consumers maintain visits/read marks from those events (`server/internal/sidecar/mirrors.go:92-139`). Drain lifecycle work and define overflow reconciliation; do not blindly make every callback synchronous under the state lock.
3. **Encode each broadcast once.** `server/internal/sidecar/hub.go:669-695` marshals the same event per recipient. Share immutable encoded bytes while preserving per-recipient lag/resync behavior. Profile before treating this as a leading bottleneck.
4. **Sort/group sidebar users once per state update.** `usersIn` sorts all users before filtering, and the sidebar calls it per channel (`packages/protocol/src/selectors.ts:37-41`; `apps/web/src/components/ChannelSidebar.svelte:68-69`). Group a once-sorted list instead of repeating whole-user sorts.
5. **Use focused wire-contract fixtures across Go and TypeScript.** Shared event examples and boundary tests would catch drift and sentinel interpretation errors without introducing a large schema-generation framework. R5 is a concrete reason to formalize boundary normalization.

## Comparison with the first report

| Initial finding | Independent conclusion |
| --- | --- |
| 1. Greeting accumulation | Confirmed High; enforce a cumulative byte budget. |
| 2. Bootstrap outbox overflow | Confirmed defect; downgrade to Medium capacity/reliability. |
| 3. Unregistration identity collision | Confirmed Medium; normalize both Go and TS boundaries. |
| 4. Concurrent quota bypass | Confirmed Medium; shared atomic reservations. |
| 5. Quadratic reaction ordering | Confirmed Medium; compute grouped ordering once. |
| 6. Unbounded stored chat | Confirmed Medium; caching is not retention/windowing. |
| 7. Reused message IDs | Confirmed Medium; read cursors require non-reused IDs. |
| 8. VoiceTarget limiter bypass | Confirmed Medium, with workload-dependent impact and a suitable burst budget. |
| 9. Desktop TLS stall | Confirmed Medium from source; still lacks a runtime stall test. |

Additional actionable findings are R1, R3, R4 and R13. Existing positives also stand: hashed cookie-session storage, origin/JSON controls, upload/download hardening, dial-time SSRF protection and bounded preview fetching, deployment CSPs, desktop certificate trust/pins, and extensive tests. This was not a cryptographic audit.

## Lower-priority and conditional observations

- **Upstream TLS deployment tradeoff:** `deploy/compose.murmur.yml:14-17` disables verification by default for documented self-signed-server use. Remote untrusted networks need verified TLS or explicit CA/pin trust; this is a deployment hardening concern, not a new universal exploit.
- **Stale channel descriptions:** hash changes can retain old text because fetch tracking uses channel ID rather than `(channel, hash)` (`packages/protocol/src/state.ts:168-183`; `packages/protocol/src/selectors.ts:67-71`; `apps/web/src/lib/connection.svelte.ts:80-87`). Follow the avatar invalidation pattern.
- **Conditional ACL authorization race:** dynamic-group permission such as `@in` can be lost through a move during an unlocked database lookup (`server/internal/mumbled/perms.go:146-183`). Recheck permission after reacquiring the lock. Disconnected users are already rejected, and `saveMu` serializes ordinary concurrent ACL edits; this is not the broader disconnected-writer flaw considered during review.
- **No default-CSP tracking claim:** retained inline CSS in sanitized Mumble HTML is only a hardening concern for custom hosting without the expected CSP. Default web/desktop CSPs block the relevant external image/font/style fetches.

## Verification and limitations

### Additional validation for this review

- Temporary Go overlay tests covered sanitizer scaling, guest-comment global-lock blocking, listener revocation in state and through local TLS clients, and logout-all with an established socket. All five passed as defect reproductions; the recording agent independently reran them.
- The capture failure proof compiled the actual source with mocked browser media APIs and asserted that no track was stopped after the failed start/stop/close sequence; independently rerun successfully.
- These tests describe current defects, not fixed behavior. Artifacts stayed outside the checkout; no application or test source was added to the repository.

### Baseline checks from the first audit, at the same application revision

- `pnpm test`: passed (121 protocol tests, 244 web tests, Go suite).
- `pnpm typecheck`: passed, including Go vet; Svelte reported zero errors/warnings.
- `pnpm --filter @kallisti/web build`: passed.
- `cd server && go test -race -count=1 ./...`: passed, uncached.
- Desktop `cargo test --locked`: 31 unit tests passed, one ignored; 11 integration tests reported passed, but real-Murmur bodies were environment-gated and not enabled.
- `pnpm audit --prod --json`: zero production JavaScript advisories reported at review time.
- Temporary reproductions covered the initial nine findings except the source-only desktop TLS stall; performance numbers are local synthetic measurements, not production load results.

### Boundaries and disposition

- Source review and targeted local tests, not a penetration test, fuzz campaign, production load test or hardware/browser audio assessment. No live deployment or private `.env` was accessed.
- No official Mumble-client harness, gated real-Murmur integration bodies or Go advisory scan. Rust advisory scanning was unavailable because the mise `cargo-audit` shim had no configured version.
- No implementation/TDD cycle was appropriate for a review-only request; existing checks and defect reproductions supplied verification instead.
- The independent audit is complete and its tracking issue is archived. **All application findings remain unfixed.** Archiving records delivery of the report, not remediation.
