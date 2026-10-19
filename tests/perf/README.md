# Perf: audio after start

A manual measurement, not run in CI. It checks how soon and how smoothly
a listener hears voice after joining a busy voice channel, on a slow
link. It is the setup behind the DEVLOG entry "Audio after start
(2026-10-19)".

`run.sh` starts:

- a fresh built-in server (`server/cmd/server`) on its own work dir, with
  400 messages in "general" (every 4th with a 70 KB JPEG, `seed.mjs`);
- `warmload` (`server/cmd/warmload`): N registered Mumble users with
  random-noise avatars (50-120 KB each) in "General", the first one
  talking without pause (`tone.opus`, 10 s of 20 ms Opus frames, looped);
- two `linkproxy` instances (`server/cmd/linkproxy`): all connections
  share one bottleneck, 8 or 3 Mbit/s down, 2 Mbit/s up, 20 ms each way.
  Chromium's own throttling is per request, so it does not show the
  socket and HTTP competing for the link.

Then for each scenario `measure.mjs` opens the app in a headless Chromium
with a fresh profile (raw DevTools protocol, no npm packages), logs in as
a guest and joins General at once. It records each socket message (type,
bytes, arrival), the playback counters every 100 ms and long tasks.
`analyze.mjs` summarizes the files.

The playback counters come from a small hook in the web client
(`apps/web/src/lib/perfHook.ts`): when the page defines
`window.__kallistiPerf` before the app loads (measure.mjs does), the app
puts `playback()` on it. Without that object it does nothing.

## Prerequisites

- Go (the version in `server/go.mod`), Node 22 or newer, pnpm.
- ffmpeg and sqlite3 on PATH, for the seed (or set `FFMPEG`, `SQLITE3`).
- A headless Chromium in `CHROME`. The simplest is Playwright's headless
  shell:

  ```sh
  npx playwright install --only-shell chromium
  # macOS (arm64); the newest revision:
  export CHROME=$(ls -d ~/Library/Caches/ms-playwright/chromium_headless_shell-*/chrome-headless-shell-mac-arm64/chrome-headless-shell | tail -1)
  # Linux:
  export CHROME=$(ls -d ~/.cache/ms-playwright/chromium_headless_shell-*/chrome-headless-shell-linux64/chrome-headless-shell | tail -1)
  ```

  A normal Chrome or Chromium binary also works: measure.mjs always
  passes `--headless`, so no window opens.

## Steps

```sh
pnpm install
pnpm --filter @kallisti/web build          # the server serves apps/web/dist
CHROME=... tests/perf/run.sh before        # label for the result files
```

A full run (7 scenarios of 15 s) takes about 3 minutes. A quick check:

```sh
CHROME=... USERS=5 DURATION=10 SCENARIOS=local tests/perf/run.sh smoke
```

To compare two versions, build each web app and server, and run with a
different label each time (use `WEB_DIR` for a web app in another place).

Settings (environment):

| Variable | Default | Meaning |
| --- | --- | --- |
| `WORK` | `$TMPDIR/kallisti-perf` | binaries, database, logs, `results/` |
| `SCENARIOS` | `8m 8m 8m:4 3m 3m 3m:4 local:4` | runs: `<link>[:<CPU slowdown>]`, link `8m`, `3m` or `local` |
| `USERS` | 25 | warmload users |
| `DURATION` | 15 | seconds per run after the join |
| `SEED` | 1 | 0: no history |
| `WEB_DIR` | `apps/web/dist` | web app to serve |
| `PORT` | 18101 | server HTTP (`local`) |
| `PROXY_8M_PORT`, `PROXY_3M_PORT` | 18102, 18103 | proxies |
| `MUMBLE_PORT` | 64820 | server Mumble port (warmload) |

The database and uploads are new for each run. The warmload certificates
stay in `$WORK/load`. The server's HTTP port listens on all interfaces
(the Mumble port and the proxies only on 127.0.0.1). run.sh gives
warmload a fixed time (5 s plus 0.1 s per user) to connect its users; it
does not check that all of them are in.

## Output

`run.sh` prints one row per run and writes the details (bytes per message
type and per second, voice gaps, late counter per second) to
`$WORK/results/<label>.txt`. The columns:

- voice s: first voice frame, seconds after the click on General;
- late: late (dropped) audio chunks in the first 10 s of voice;
- last late s: the last new late chunk, seconds after the first voice
  ("-": none);
- blob KB: avatar textures on the socket (RequestBlob answers), first 10 s;
- max gap ms: the longest gap between voice frames (20 ms when smooth);
- long ms: long tasks on the main thread in the first 10 s, in all.

The JSON files stay in `$WORK/results`; `node analyze.mjs [--short]
<files>` reads them again.

## Reference numbers

From the DEVLOG: 25 users, a Mac, Chromium headless shell 1228. "Before"
is the code before the merge 3435848 (b7d645e); that client has no perf
hook, so the late counts need the hook line added to
`apps/web/src/lib/connection.svelte.ts` by hand.

| Scenario | Before: voice s | Before: late | After: voice s | After: late |
| --- | --- | --- | --- | --- |
| 8 Mbit/s | 2.2 | 96 | 0.08 | 0 |
| 3 Mbit/s | 5.6 | 270 | 0.15 | 0 (one run: 1) |
| local, CPU x4 | 0.6 | 19 | - | 0 (also x6) |

Before, 2.0 MB of avatar blobs went over the socket in the first 2.3 s,
ahead of the voice. After, the avatars come over HTTP and blob KB is 0.
Long tasks after: 140-170 ms (CPU x1), 370-640 ms (x4-x6).

Smoke runs (5 users, 10 s, after the fix): `local` 0.02 s and 0 late
(another run: 3, all in the first second), `8m` 0.08 s and 0 late,
`3m:4` 0.15 s and 0 late.
