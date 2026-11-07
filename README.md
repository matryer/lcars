# LCARS

A file-fed canvas with a TNG-inspired interface. One Go binary provides the shell, navigation, panel frames, live refresh, update times, isolated HTML, and screenshot tooling. Agents publish Markdown, text, and ordinary HTML files; LCARS can run opt-in adjacent data scripts; it has no agent runtime, built-in upstream integrations, or chart/calendar engine.

## Build and run

Go is pinned in `mise.toml`. Run `mise install` to install it, then:

```sh
mise run build
./lcars validate --data ./examples/content
./lcars serve --data ./examples/content
```

The server binds to loopback on port 8080 by default. The included content is explicitly labelled demo information. For a fuller example with screenshots, see [examples/machine](examples/machine/README.md): an illustrative machine-stats canvas with charts, a sortable table and live data files. For your own canvas, create an empty data directory and run `lcars serve --data /path/to/content`. The root must exist; the app does not create or populate it for you.

```text
content/
  overview/
    briefing.md
    energy.html
    assets/
      energy/
        data.json
```

Each immediate folder is a tab at `/s/{tab}`. Direct `.html`, `.md`, and `.txt` files become panels. HTML supplies an interior; the host supplies the frame. Live `assets/{panel}/data.json` updates an HTML panel without resetting its interaction state. Panels and resources are private content and should be treated accordingly.

Agents can discover the bundled skill directly from CLI help:

```sh
./lcars skill                       # print SKILL.md
./lcars skill --guide               # print the canonical guide
./lcars skill --starter > panel.html # print the HTML starter
./lcars skill --out ./lcars-authoring # export the whole portable folder
./lcars version --json              # app build, contract and bridge versions
```

Export requires a new destination directory and never overwrites an existing skill. Nothing installs automatically.

Read the [authoring guide](skills/lcars-authoring/references/guide.md), also served at `/agent-guide`. The portable [lcars-authoring skill](skills/lcars-authoring/SKILL.md) and [starter](skills/lcars-authoring/assets/starter.html) describe the version 1.0 contract. Copy the entire skill directory explicitly when installing it into a supported agent environment; nothing installs itself.

## Capture and verify

The server needs no browser or Node runtime. The capture commands need installed Chrome/Chromium, with `--browser` for a custom executable:

For one command that validates the directory and captures every section at desktop, tablet, mobile, and 320px widths:

```sh
./lcars snapshot --data ./examples/content --out ./captures
```

`snapshot` starts a temporary loopback server only for capture and closes it when done. It does not publish or install a service. Use `screenshot` against an existing service for an individual view or hash-checked panel:

```sh
./lcars screenshot --url http://127.0.0.1:8080/s/overview --width 1440 --height 900 --out desktop.png
./lcars screenshot --url http://127.0.0.1:8080/s/overview --width 390 --height 844 --out mobile.png
./lcars screenshot --url http://127.0.0.1:8080/s/overview --panel overview/energy --out energy.png
```

These loopback URLs are local diagnostic examples. A capture writes a PNG and neighbouring JSON report. Use `--panel` with `--expect-panel-hash` and `--expect-data-hash` to require hashes printed by `validate`; absent data uses `missing`. Capture checks acknowledged displayed revisions, freezes the clock and optional motion, and records browser/resource/overflow errors. A passing capture still requires visual inspection.

Run checks with:

```sh
mise run test
```

## Publishing model

Write hidden temporary files on the same filesystem, then atomically rename them. Stage resource files before replacing the document with changed bytes; the document is the bundle commit point. Live `data.json` publishes separately. Invalid changes retain accepted content and display a diagnostic. Unchanged-byte writes do not refresh timestamps. The UI exposes publication and producer observation times separately.

The scanner polls disk every second; accepted changes notify visible browsers through SSE, which fetch current state with ETags. Browsers recover on reconnect and fall back to polling if SSE is unavailable. Script publication triggers an immediate scan. HTML runs in opaque-origin `sandbox="allow-scripts"` frames with restricted resources and a narrow per-instance message bridge. This isolates mostly trusted agent code; it does not provide hostile-code CPU/memory containment or guarantee complete network containment.

## Scheduled data scripts

Put `weather.5m.py` beside `weather.html`. The filename identifies the panel and interval; successful JSON stdout is validated and published to `assets/weather/data.json`. Panels receive it through the existing `LCARS.onData()` callback without a reload.

```sh
./lcars serve --data ./content --run-scripts
```

Scripts are trusted local programs, enabled only by this flag. Use positive whole-number intervals in seconds (`s`), minutes (`m`), hours (`h`), or days (`d`). Hide a script with a leading dot to disable it. One producer per HTML panel; helpers without an interval are not scheduled. Sources are never served.

Installed interpreters are required: Python (`.py`), Node (`.js`), POSIX shell (`.sh`), or PowerShell (`.ps1`). Override executable paths with `--script-py`, `--script-js`, `--script-sh`, and `--script-ps1`. Defaults: 30-second timeout (`--script-timeout`), four concurrent jobs (`--script-concurrency`), no overlapping runs, and no missed-run replay. Scripts start once at startup and wait their interval after each completion. The runner owns a process-lifetime lock per content root; the small hidden lock file intentionally remains after exit.

Only basic execution environment variables are inherited. Pass additional credentials/configuration explicitly with `--script-env NAME,OTHER_NAME`; never print secrets. stdout must contain one existing version-1 data envelope, at most 256 KiB. stderr is retained only in bounded service logs. Failed runs preserve data and show a panel diagnostic. Observation timestamps remain producer-supplied. See the [authoring guide](skills/lcars-authoring/references/guide.md#schedule-an-adjacent-data-script) for a working cross-platform example.

## Private deployment

Keep the default loopback listener behind your existing private authenticated proxy. Configure proxy authentication for **all** routes, including state, `/api/events`, components, resources, toolkit, and guide. Disable buffering for `/api/events` and allow long-lived responses; the server sends heartbeat comments every 15 seconds. LCARS does not implement accounts, a public write API, TLS termination, or built-in upstream fetching. Enabled local scripts can fetch data with the service account’s permissions. Versioned resource URLs are immutable bytes, not permission for public/shared caching. Do not deploy content directories as a separate static web root.

To block DNS-rebinding attacks, LCARS answers only requests whose Host is `localhost`, an IP address, or a name passed to `--allowed-host`. If your proxy forwards the browser's Host header (as `tailscale serve` and many authenticating proxies do), list that name, for example `--allowed-host lcars.example.ts.net`. Proxies that rewrite Host to the upstream address need no flag.

A process manager can run:

```sh
/path/to/lcars serve --data /absolute/path/to/content --listen 127.0.0.1:8080
```

Ordinary serving needs only read access to the root; enabled script execution also needs write access for its lock and published data. Authors need write access through existing filesystem/SSH permissions. Keep backups separately if required; the released-resource grace cache is bounded and is not revision history.

When sharing a tailnet service, discover the serving machine's full DNS hostname with `tailscale status --json`, preserve port/path, and verify the final URL before sharing. Do not assume that a loopback URL works from another device. On Kitt, service additions, removals, moves, and renames must also update the private homepage registry in `/home/matryer/code/github.com/matryer/devtools` under that repository's instructions. Running local diagnostics does not require publishing a new service.

## Versions and local releases

`internal/buildinfo/VERSION` is the single app version source (SemVer, initially `0.1.0`). Ordinary `go build` embeds it; `lcars version` reports the build, commit, dirty state, Go version, and platform. The private `/api/version` route reports the same JSON. Capture reports identify the capture binary too. App releases are versioned separately from the stable authoring contract `1.0`, bridge `1`, and JSON schemas `1`; an app patch does not change those contracts.

```sh
mise run build                         # build ./lcars
mise run test                          # race tests and vet, including browser tests if installed
mise run bump                          # bump patch only (0.1.0 → 0.1.1)
mise run bump --bump minor              # bump minor and reset patch
mise run release                       # bump patch, test and package
mise run release --bump none           # test and package the current version
mise run release --bump major          # bump major and reset minor/patch
mise run release --targets linux/amd64 # one platform for a quick local release
```

The equivalent release command is `go run ./tools/release` (patch bump by default); `--bump none` packages the current version, and `--version-only` only bumps VERSION. Default release targets are Linux and macOS on amd64/arm64 and Windows on amd64. Packaging needs Go and Git; Chrome/Chromium is only needed for browser checks, which skip when unavailable.

A release runs race tests and vet before building static binaries. Each `dist/vVERSION/` contains platform archives, `SHA256SUMS`, and `release.json` with the version, source commit, dirty state and build time. Every archive includes the executable, README, font licence, and complete portable authoring skill. Builds embed the version, commit, and build time. Existing release directories are never overwritten; a failed build restores VERSION and leaves no completed release folder.

Review and commit the finished source and VERSION, then run `mise run release --bump none` from that clean commit for publishable artifacts. Commit the bump first with `mise run bump` if you want the release's source commit to include it. The tool can build a dirty tree for local use; inspect `lcars version --json` before publishing. Tag `vVERSION` and upload the archives/checksums through your normal release process when ready. These commands do not commit, tag, push, or publish for you.

## Source layout and inspiration

`cmd/lcars` contains the CLI; `internal/content` handles validation and accepted snapshots; `internal/server` serves the shell, state, SSE, and immutable bundles; `internal/scripts` runs scheduled producers; `internal/capture` drives Chromium; `web` contains embedded browser assets and local fonts. The canonical authoring guide and starter live with the portable skill and are embedded in the binary.

LCARS is inspired by the Star Trek interface designed by Michael and Denise Okuda and the production teams of *Star Trek: The Next Generation*. It is an independent implementation, not an official Star Trek product. Geometry is original; episode captures and copied ship artwork are not bundled. Antonio is distributed under its bundled SIL Open Font License. Design research and scope are recorded in [PLAN.md](PLAN.md).
