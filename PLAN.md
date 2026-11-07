# LCARS: a simple agent-fed canvas

Status: implemented v1. The application, portable authoring skill, demo content, and focused regression tests now live in this repository. `lcars snapshot --data ./content --out ./captures` combines validation and temporary-server desktop, tablet, mobile, and narrow capture. The design and contracts below describe the v1 scope.

## 1. The app

Build one Go binary that serves a calm, useful, TNG-inspired web interface. Agents write ordinary files; LCARS supplies the shell, layout, shared styling, live refresh, update timestamps, and screenshot tooling.

Folders are tabs. HTML files are interactive panels. Markdown and plain text are prose panels. Agents can make calendars, briefings, diagrams, tables, or charts using ordinary HTML, CSS, JavaScript, SVG, and Canvas. The app has no built-in calendar, chart engine, JSON widget language, model, agent runtime, upstream integrations, or scheduler in the original v1 scope. The extension below adds opt-in scheduled scripts.

The useful loop is: ask an agent for content → write files → validate → inspect desktop and mobile screenshots → revise. Agents use their existing authorized tools to collect information. LCARS displays it; upstream actions such as sending email or changing calendar events remain outside this app.

## 2. Visual research and direction

Research reviewed on 7 November 2026:

- [TNG, “The Hunted”: screen reference](https://www.ex-astris-scientia.org/observations/thehunted/09-thehunted-r.jpg), with [episode context](https://www.ex-astris-scientia.org/observations/thehunted.htm). The inspected image has broad pale elbows, black separating gaps, rounded control ends, tightly condensed labels, and large black working areas. Borrow the proportions and hierarchy, not the camera distortion or tiny lettering.
- [Vessel-status screen reference](https://gtjlcars.de/images/LCARS-Entstehung/twisted246.jpg), from [LCARS history](https://gtjlcars.de/LCARSindex/Entstehung.htm). The inspected image separates a large technical schematic from dense supporting readouts using coloured rails. Use that relationship for home diagrams and telemetry. This is a broader franchise reference, not evidence of one exact TNG palette.
- [Mike and Denise Okuda interview](https://trektoday.com/interviews/okuda_qa.shtml). Their explanation emphasizes organization and the difference between convincing screen graphics and usable software. Practical consequence: actual labels, meaningful values, predictable interactions, and readable prose.
- [TheLCARS.com](https://www.thelcars.com/) demonstrates a responsive HTML/CSS interpretation. Treat it as secondary inspiration, not a dependency or a source of code to copy.
- [Antonio font licence](https://raw.githubusercontent.com/google/fonts/main/ofl/antonio/OFL.txt): SIL OFL. Bundle Antonio and its licence for headings and navigation. It is our practical condensed-font choice, not a claim of exact screen typography.

The result should feel like a working Enterprise terminal: black canvas, continuous asymmetric framing, warm apricot/orange structure, lilac secondary structure, occasional blue data, and purposeful LCARS-style animation. Avoid generic rounded dashboard cards with an LCARS colour wash. Avoid gradients, glass, shadows, glowing text, starfield wallpapers, decorative fake telemetry, and illegible serial numbers.

Screen imagery is research only; do not bundle episode captures or copied ship art. Draw original geometry. Credit LCARS' Star Trek inspiration in project documentation.

### Design tokens

These are proposed web colours, not purported canonical sampled values. Final contrast and visual QA must validate them.

| Token | Value | Role |
|---|---|---|
| `background` | `#000000` | Canvas and panel interior |
| `text` | `#F5EEE6` | Body text |
| `muted` | `#B8ADB7` | Secondary text and timestamps |
| `apricot` | `#FFCC99` | Primary shell and navigation |
| `orange` | `#FF9900` | Active section and emphasis |
| `lilac` | `#CC99CC` | Secondary panel framing |
| `blue` | `#9999FF` | Secondary data series |
| `peach` | `#FF9966` | Additional series and accents |
| `critical` | `#FF6666` | Explicit error/critical status |
| `ink` | `#000000` | Text inside coloured bars |

Normal status is written as “Normal”, not inferred from a colour. Warning uses orange plus “Warning”; critical uses red plus “Critical”. Panel accents identify sections, not health. Charts also distinguish series with labels and dash patterns. Never use white labels on light pastel bands.

Bundle Antonio for short uppercase labels, headings, and large values. Use the system sans-serif stack for mixed-case email, explanations, and tables; use tabular numerals where supported. Body text starts at 16px / 1.5 line height, labels at 18px, headings at 24–32px, key values at 36–56px. No horizontal font scaling or all-uppercase paragraphs. Ship local fonts so the interface works without external network access.

Spacing uses a 4px base, usually 8/16/24px. Desktop shell rail: 176px; shell elbow outer radius: 64px; frame band: 24px; black segment gaps: 8px. Compact panel elbow: 32px outer radius with 12px band, inner radius derived by subtraction. Titles occupy a black interruption in the upper band, outside the elbow's turning area. Use CSS geometry and pseudo-elements; titles remain real selectable text.

## 3. Files and layout

```text
content/
  overview/
    briefing.md
    calendar.html
    assets/
      calendar/
        data.json
  home/
    energy.html
    notes.txt
    assets/
      energy/
        data.json
        diagram.svg
```

Each immediate ordinary directory is a tab at `/s/{tab}`. An ordinary directory inside a tab (other than `assets/`) is a sub-tab at `/s/{tab}/{sub}` with the same rules, its own optional `_section.json` and `assets/`; deeper folders are diagnosed. The tab's own panels form its first sub-tab; a tab with only sub-tabs opens the first. Panel IDs are `tab/panel` or `tab/sub/panel`, and component URLs encode the ID as one path segment (`tab~sub~panel`). Each `.html`, `.md`, or `.txt` file directly inside it is a panel. `assets/` contains resources and never creates panels. Ignore hidden and temporary files; warn about unsupported visible files and directories. JSON is live data, not a panel format.

Tab and panel IDs use lowercase letters, digits, and hyphens, 1–64 characters. A panel's identity is `tab/filename-stem`; duplicate stems across formats are errors. Labels default to filenames with hyphens replaced by spaces. Sort tabs by folder name and panels by order, then ID. Optional tab `_section.json` accepts only `version: 1`, `title`, `order` (default 100), and a named palette `accent` (default lilac); tabs sort by that order first. No registry, per-panel sidecar, or site configuration is required.

HTML uses `<title>` for the frame title. Optional `lcars:size` metadata accepts `normal`, `wide`, or `full`; `lcars:order` is an integer, default 100; `lcars:accent` is a named palette colour. Optional `lcars:updatedAt` is the producer's RFC3339 observation time, and `lcars:staleAfterSeconds` is a positive integer requiring that timestamp. Validate recognized metadata. Accept complete HTML documents or normalize fragments.

For Markdown, a first level-one heading supplies the panel title and is omitted from the body. Support paragraphs, headings, lists, emphasis, links, and code through a maintained parser. Disable raw HTML and remote images. Local PNG, JPEG, WebP, and passive SVG images must live under the panel's own `assets/{id}/`, have descriptive alt text, and preserve aspect ratio. Plain text preserves line breaks. Text panels need no metadata; use HTML when custom layout or observation metadata is needed.

The shell owns navigation, a skip link, main landmark, and consistent panel frames. Active navigation uses `aria-current` and a visible marker. The desktop rail is replaced by an accessible compact menu below a 760px viewport. Navigation uses ordinary section links and browser back/forward; there is no custom router, drill-down URL state, or transition coordinator. Root `/` redirects to the first tab or shows a welcome screen. Missing/deleted sections show a labelled state with a link to an available section.

Use CSS Grid based on content width: one column below 720px, two below 960px, otherwise three. Normal panels span one column, wide panels up to two, full panels all columns. Preserve DOM order; no dense packing or masonry. Let content determine row heights. Titles wrap, long URLs break, panels have `min-width: 0`, and tables can scroll within their panel. Never create page-level horizontal overflow or shrink prose to fit.

### Component sizing contract

Be prescriptive about fitting the space, while allowing authors to choose Flexbox or Grid. A component responds to its actual allocated width, not the outer browser width or the assumption that `wide` means desktop. Inside an iframe, viewport media queries refer to that frame; container queries are optional for internal regions. Start with a single-column layout and add columns only when the content fits. The host sets width and the component supplies intrinsic height; do not use fixed page widths, viewport-based heights, or absolute positioning for the main layout.

The injected base styles and HTML starter provide these defaults:

```css
*, *::before, *::after { box-sizing: border-box; }
html, body { margin: 0; min-width: 0; }
.lcars-content { min-width: 0; overflow-wrap: anywhere; }
.lcars-row { display: flex; flex-wrap: wrap; gap: 1rem; }
.lcars-row > * { min-width: 0; max-width: 100%; }
.lcars-stack { display: grid; grid-template-columns: minmax(0, 1fr); gap: 1rem; }
img, svg, canvas { display: block; max-width: 100%; }
img { height: auto; }
```

The helper's measured content wrapper uses `.lcars-content`. Authors can use or adapt the row/stack classes; they are layout conveniences, not another component API. Grid columns use shrinkable tracks such as `minmax(0, 1fr)` and flex children must be allowed to shrink. SVG diagrams need a `viewBox` and an intentional aspect ratio. Canvas charts resize their drawing buffer and redraw for the allocated width; scaling a desktop bitmap is insufficient. Avoid tiny labels by reflowing legends, reducing tick density, or supplying an accessible data disclosure.

At narrow widths, wrap controls, stack label/value groups, and choose a useful compact presentation. For example, an agent-generated calendar may become an agenda instead of squeezing seven columns onto a phone. Keep the same underlying information and selection state when reflowing. Touch and keyboard users must be able to reach details without hover. Dates, update timestamps, and primary actions remain visible; do not hide essential content to make a screenshot fit.

Horizontal scrolling is an explicit exception for genuinely two-dimensional material such as a wide table, contained in a labelled, keyboard-accessible region with a visible overflow affordance. Code blocks may also scroll locally. Do not fix overflow with global `overflow: hidden`, clipped content, or transform scaling. All other content should reflow, including long unbroken text and translated/expanded labels.

## 4. Last updated, everywhere

Every panel frame shows a single compact relative time such as `5m` or `2h`, supplied by the host even for metadata-free text. A keyboard- and touch-accessible disclosure exposes the exact date, time, timezone, and timestamp source. Use semantic `<time>` elements, readable muted text, and the viewer's timezone. Recompute relative labels once per minute without screen-reader announcements.

`Updated` means the displayed content revision's file update time: snapshot the latest modification time of its document, included assets, and acknowledged live data. Only successfully displayed changes advance this value. Polling, rejected changes, pending replacement frames, and unchanged-byte writes do not refresh it. Keep these times with accepted hashes in memory; on startup use file times, never startup time. File times are best-effort publishing evidence, not an audit history: externally touching files before a restart can change them. Unknown times show `—`; future times show the absolute time with a clock-skew diagnostic.

Producer `updatedAt` means when the underlying information was observed. When known, the visible time is the observation age instead of the publication age; the disclosure lists both exact times, or `Observation time unknown`. If its stale threshold expires, visibly show `Stale` and the observation age. Changing a heading must not make old measurements look fresh. Rejected updates retain the displayed revision's timestamp; rendering failures label it as last verified in the disclosure. Deleting live data shows `Data unavailable`, without inventing a file timestamp.

Each section header shows `Latest content update …` for its displayed panels, explicitly the newest update rather than a claim that every panel is fresh. Empty sections show `No content yet`. The shell footer reports connection status and the last successful check separately from content freshness. No per-cell timestamps or update-history system.

## 5. Publishing and refresh

Files are the only publishing interface. One agent owns each panel and its asset directory. Write hidden temporary files on the same filesystem, then atomically rename them over their targets. Different agents can publish different panels independently; same-path replacements are last-writer-wins.

```sh
mkdir -p content/overview
printf '%s\n' '# Briefing' '' 'Two emails need a reply.' > content/overview/.briefing.md.tmp
mv content/overview/.briefing.md.tmp content/overview/briefing.md
lcars validate --data ./content
```

For code/resources, stage assets first, then replace the panel document with changed bytes as the commit point. A comment change is enough to publish resource-only changes. When document bytes change, snapshot the document and its complete `assets/{id}/` directory, excluding live `data.json`, hidden files, and temporary files. Accepted bundles are immutable; later asset-only edits do not alter them. This requires the writer to finish staging before committing, not a multi-file transaction system. Failed candidates are retried on later scans.

Scan every second, hash bytes rather than relying on modification times, and publish an internally consistent state once per scan. Invalid content retains its last accepted revision with `Update rejected`; a first-time failure gets a labelled placeholder. Missing required assets reject that revision. Invalid tab metadata retains prior settings or defaults with a diagnostic. Confirm deletions only after successful directory enumeration; unreadable directories or root loss retain prior content with an error. Require the data root at startup. Renames are removal plus discovery.

Keep only the current accepted bundle per panel. Released resource bytes have a 60-second, 100 MiB maximum grace cache, evicted oldest first. There is no previous-revision pinning, rollback API, or per-browser retention. Already-loaded old frames may remain visible after a failed replacement; expired lazy-resource requests report an error and offer Retry latest. Reloads and restarts use current disk content, not historical browser state.

Limits: 200 panels; 1 MiB per HTML file; 256 KiB per text document or live-data envelope; 10 MiB per resource; 100 MiB total unique current asset bytes. Check size before reading large files. Apply confirmed deletions first, then admit changed panels in bytewise full-ID order against the resulting budget. Reject over-budget candidates without evicting accepted panels. `validate` uses the same validation and cold-start admission rules, reports actionable paths and limits, and exits nonzero for errors. It prints document and live-data SHA-256 hashes for screenshot checks. It does not execute scripts or prove visual quality.

The browser subscribes to state revisions through SSE and fetches current state with ETags. It pauses while hidden and refreshes on reconnect or return. Unchanged scans do not change the state revision. Failed requests retain content, show disconnected status, and back off to at most 30 seconds; polling is the fallback when SSE is unavailable. External edits retain the one-second scan delay; script publication triggers an immediate scan. Patch only changed panels; avoid rebuilding navigation or unrelated content.

## 6. HTML and live data

Inject a small versioned stylesheet and helper before author scripts. Supply palette, font, and spacing variables; the host draws the outside frame. Bundle fonts locally, including embedded font resources for isolated frames. Agents own the interior, without a framework or frontend build requirement. Use local classic script/style bundles; no CDN, dynamic imports, workers, or network fetching in components.

An optional `assets/{panel-id}/data.json` feeds the matching HTML panel:

```json
{
  "version": 1,
  "updatedAt": "2026-11-07T09:00:00Z",
  "staleAfterSeconds": 300,
  "data": {"watts": 420}
}
```

Require `version` and `data`; observation fields are optional with the same rules as HTML metadata. Reject unknown envelope fields; the inner `data` is arbitrary JSON owned by the author. Its observation metadata takes precedence over HTML metadata while a data file exists. Malformed updates retain accepted data with a diagnostic. Absence/deletion delivers an explicit missing state, distinct from a present envelope whose data is null. Never substitute zero for unavailable values.

The small `window.LCARS` helper provides:

- `onData(callback)`: replay the initial envelope after handshake, then deliver changed data without replacing the frame. Include `missing` in the delivery. Return an unsubscribe function; await callback promises before acknowledging the rendered hash. Serialize callbacks and coalesce pending ordinary updates to the latest value.
- `onState(callback)`: provide visibility, motion preference, capture status, and the display clock. Stop optional animation when hidden or motion is off. Initial rendering must work while hidden and must not depend on animation ticks.
- `deferReady()`: called during initial execution, return a completion function for asynchronous initialization. Ordinary DOM, fonts, images, and initial-data readiness are handled automatically.
- `onCapture(callback)`: draw a still for the supplied frozen display clock, awaiting any returned promise. No timeline controls or animation recording.
- `openDetail({title, body, fields})`: show a text-only detail view inside the panel's own frame. The toolkit hides the panel content without destroying it (state, scroll and listeners survive), draws the title, paragraphs and fields with text nodes, focuses the title, and goes back on its Back button or Escape, restoring the content and returning focus to the opener. A new detail replaces an open one; captures close it. Resolves `true` on Back, `false` if replaced or not opened. Fields carry a `value` or an RFC3339 `time`. Bridge: frame→host `detail {title|null}` (the host validates it and, for the displayed frame only, shows a breadcrumb `PANEL › DETAIL` and a Back control on the frame's top bar); host→frame `back`. `openModal` remains a contract-v1 alias of `openDetail`.
- `navigate(section)`: ask the host to open a section or sub-section; accepted only from the displayed frame the user is interacting with (user activation in the frame, iframe focused in the shell), never during capture, and only for a section that exists; resolves `true` or `false`.
- `<lcars-time datetime label>`: compact relative time (`5m`, `2h`, `in 3d`) driven by the display clock; pressing it opens the exact time in a detail view. The frame's own update time is a host button that toggles an inline row of exact Updated and Observed times under the panel.
- `.lcars-scan` / `.lcars-scan-radial` (toolkit CSS): a periodic scanning band (or radar sweep) over a visualisation, transform and opacity only, running only with motion on while the panel is visible (`html[data-motion=on][data-visible=true]`).

A minimal author script is ordinary JavaScript:

```html
<output id="power">Unavailable</output>
<script>
  LCARS.onData(({data}) => {
    document.querySelector('#power').textContent =
      Number.isFinite(data?.watts) ? `${data.watts} W` : 'Unavailable';
  });
</script>
```

Measure intrinsic content height with ResizeObserver and coalesce reports per frame. Start at 320px, clamp to 80–2400px, and provide an internal scroller with a diagnostic above that limit. Ignore changes under 1px; diagnose oscillation. Components use fluid width and avoid viewport-height rules that feed back into iframe sizing.

Serve HTML in `sandbox="allow-scripts"` frames without same-origin, forms, popups, downloads, or top-navigation privileges. Apply a sandbox CSP to component documents and every asset response. Restrict resources to the server's own origin using `'self'`, never the request Host, so Host-rewriting proxies work; disable connections, nested frames, and object/embed. Validate paths and resource types; refuse symlinks and traversal. Passive SVG images use a documented allowlist rejecting scripts, animation, and external resources. This is isolation for mostly trusted agent code, not a hostile-code CPU/memory or complete network-containment guarantee.

Use a versioned bridge with per-instance nonce, exact window-source checks, validated bounded messages, and stale-message rejection. Its operations are sizing, lifecycle/data delivery, readiness acknowledgements, and diagnostics. It offers no arbitrary host commands. Render source links in Markdown or through an optional HTML `lcars:sourceUrl` metadata field validated as HTTP(S); source links open safely from the host. Components get only their own data.

A code update stages one replacement iframe at the correct width, hidden from interaction and accessibility but still measurable. Keep the displayed frame until the replacement acknowledges readiness and its initial data. Set a 10-second deadline; on failure dispose the candidate and retain the old display with a diagnostic. Try a failed code revision once per page session, with explicit Retry or a newer revision permitting another attempt. Superseding updates dispose earlier candidates. Ignore late acknowledgements. Keep at most one displayed and one initializing frame per panel. Dispose frames on section navigation; local interaction state may reset then or on code replacement, but data-only updates preserve it. Avoid stealing focus; when replacing the focused frame, move focus to its panel heading.

Data callbacks also have a 10-second acknowledgement deadline. A callback may mutate the DOM and then throw; the host cannot roll that back. On failure, mark the display unverified, stop deliveries to that instance, and fail captures. Retry starts a fresh instance. Hashes and timestamps always describe acknowledged displayed content, never a candidate; after failure the old hash is only last-verified evidence.

## 7. Screenshots and accessibility

Provide `lcars screenshot` using chromedp and an installed Chrome/Chromium binary. Accept `--browser` and give a clear missing-browser error. The server itself needs no browser or Node runtime.

```sh
lcars serve --data ./content --listen 127.0.0.1:8080
lcars screenshot --url http://127.0.0.1:8080/s/overview --width 1440 --height 900 --out desktop.png
lcars screenshot --url http://127.0.0.1:8080/s/overview --width 390 --height 844 --out mobile.png
lcars screenshot --url http://127.0.0.1:8080/s/overview --panel overview/calendar --out calendar.png
```

These loopback URLs are internal diagnostics. Capture the viewport by default or one panel with `--panel`. Optional `--expect-panel-hash` and `--expect-data-hash` require `--panel` and match exact file-byte hashes from validation. A missing data file uses the sentinel `missing`. Wait for the expected accepted state, latch that state in the browser, and stop live updates for capture. Require displayed readiness and data acknowledgements, not merely server acceptance. Resource expiry during capture is an error, not a reason to silently substitute a newer revision.

Freeze the display clock at capture start, turn optional motion off, wait for fonts/images/custom capture callbacks, then settle layout for two animation frames. Use device scale 1 and a 15-second overall deadline. Write a PNG plus a neighbouring JSON report with viewport, capture time, displayed document/bundle/data hashes, browser version, errors, failed resources, and overflow diagnostics. Rendering failures or hash mismatches exit nonzero even if old content remains visible. Inspect the PNG for composition; readiness checks cannot judge design quality. No server endpoint launches browsers and no public screenshot gallery is created.

Use semantic links/buttons, visible focus, 44px touch targets, readable contrast, and accessible image/chart descriptions. Long content stays available through native disclosures or scrolling where appropriate. Honor the reduced-motion preference; there is no separate motion toggle. Shell hover/focus transitions can be short; no boot sequence, background sweep, flashing warning, or continuous decorative animation. Custom motion should convey information, stop when hidden, and yield a meaningful still for capture. The app and its authored panels are silent in v1.

## 8. One authoring skill

Ship one portable `lcars-authoring` skill in the repository, alongside the app. CLI help advertises `lcars skill`, which prints the embedded skill; `--guide` and `--starter` print its supporting files, and `--out DIR` exports the complete portable folder without installing or overwriting an existing installation. It teaches how to create or update content in an existing LCARS canvas; it does not deploy the app, obtain new upstream permissions, or install itself. Agents without skill support can read the same guide directly. No MCP server is needed for v1: existing filesystem/SSH access and the CLI are sufficient.

Keep `SKILL.md` short: when to use it, where to find the guide and starter, and the publish → validate → capture → inspect loop. The canonical detailed guide lives with the skill as a supporting reference, and the app serves those same bytes at `/agent-guide`; do not maintain a second prose copy. Bundle a minimal HTML starter as an asset. Distribute the skill folder with releases for explicit copying into an agent's supported skill directory. Record the compatible app/contract version and keep examples matched to it. Deployment-specific data paths and URLs come from the environment or user, not hard-coded hostnames in the skill.

Teach only LCARS-specific decisions: choose an owned tab/panel path, use the supplied visual tokens, frame, and sizing contract, publish atomically, separate code from live data, expose observation times, handle empty/stale data, and inspect desktop/mobile captures. Require checking the panel's actual width as well as the page viewport; a one-column panel on desktop can be as narrow as a mobile panel. Preserve the user's requested presentation and source-access permissions. Do not turn a request to display calendar data into permission to edit calendar events.

Use short example briefs rather than a catalogue of built-in widgets. For example: “Show my next seven days, highlight today, make overlaps obvious, and distinguish all-day events using my local timezone.” The agent chooses and implements the presentation as an ordinary HTML panel. Calendar recurrence/source retrieval belongs to its existing tools; LCARS has no calendar integration or domain-specific event schema. Briefings, household status, and telemetry follow the same pattern. Add specialized skills only if actual use demonstrates a separate workflow worth maintaining.

## 9. Implementation and delivery

```text
cmd/lcars/                         # serve, validate, screenshot, snapshot, skill, version
internal/content/                  # parse, validate, scan, accepted snapshots
internal/server/                   # shell, state, immutable resources
internal/capture/                  # browser capture
web/                               # shell, frame helper, CSS, local fonts
skills/lcars-authoring/SKILL.md
skills/lcars-authoring/references/guide.md
skills/lcars-authoring/assets/starter.html
```

Keep the app SemVer in `internal/buildinfo/VERSION`. `lcars version --json` and the private `/api/version` endpoint identify the app build separately from authoring contract 1.0 and bridge/schema version 1. `mise run bump` increments the patch by default; `--bump minor` or `--bump major` selects another increment. `mise run release` bumps patch by default, runs checks, and creates local versioned archives, bundled skills, metadata, and checksums. `--bump none` packages the current version. Releases never commit, tag, or publish automatically.

Use Go's standard HTTP/JSON/embed libraries, small browser JavaScript, and maintained HTML/Markdown parsers. No frontend framework, database, queue, plugin engine, or bundler in the host. Escape host text and avoid injecting arbitrary values into `innerHTML`.

Themes: `classic`, `pastel`, `nebula`, `tidal`, `sand`, `meridian` recolour apricot/lilac/blue/peach for the shell and, via frame state, panel interiors; orange and critical are fixed health colours. A tab's `_section.json` may name one; otherwise FNV-1a of the top-level tab ID picks it. Decoration (rail and bar segments, filler keypads/readouts/meters/bars in grid holes left by sparse placement) is host-drawn, `aria-hidden`, unfocusable, seeded from the tab ID so it is identical across loads and captures, and animates only with motion on.

Routes: `/`, `/s/{tab}`, `/s/{tab}/{sub}`, `/api/state`, `/api/events`, `/api/version`, immutable component/asset URLs, `/toolkit/*`, `/agent-guide`, and `/healthz`. State includes accepted panel metadata, content, live data, diagnostics, and revision with ETag. Keep content responses private to the user's authenticated deployment; immutable means versioned bytes, not permission to use public shared caches. Bind to loopback by default and deploy behind the user's private authenticated proxy. Reject Host names other than `localhost`, IP addresses, and explicit `--allowed-host` values, so DNS-rebinding pages cannot read content. No upstream fetch service or public multi-user authentication system.

When deploying on a tailnet machine, discover its full DNS hostname with `tailscale status --json` and verify reachability before sharing URLs, preserving port/path. On Kitt, maintain the homepage registry in `/home/matryer/code/github.com/matryer/devtools` under its repository instructions. Planning does not launch or register a service.

Build in three usable steps:

1. **Authoring loop:** folder discovery, LCARS shell/frames, HTML and text panels, validation, file refresh, automatic timestamps, isolated frames, and basic screenshots. Inspect desktop/mobile output before expanding.
2. **Live content and recovery:** data helper, observation freshness, immutable bundles, bounded replacement/readiness handling, hash-checked capture, and an end-to-end guide/skill with a starter. Prove the loop from an empty content directory.
3. **Release checks:** responsive and keyboard behavior, motion preferences, private deployment instructions, and focused tests of validation, paths, timestamps, data updates, rejected edits, deletion, resource expiry, and capture failure.

Acceptance: an agent can create and refine a useful panel without changing app code; independent panels update without disturbing others; live data preserves local interaction state; bad input stays visible as an error without erasing accepted content; every panel exposes its update time; captures verify the displayed revision. Check desktop (1440×900), tablet (1024×768), mobile (390×844), and 200% zoom, including long titles, missing values, empty data, focus, and overflow. Test section links, reload, and browser back/forward. Use local fixtures; no upstream account is needed for app tests.

Responsive checks also cover a 320px page viewport and actual component widths around 260px, 360px, and 720px in a browser test fixture. Resize an already-mounted interactive panel to verify reflow without lost state, clipped focus, resize loops, or stale Canvas dimensions. Capture diagnostics inspect horizontal overflow inside frames as well as the shell, distinguishing declared local scroll regions from accidental document overflow. Check long words, large values, wrapped controls, portrait/landscape changes, and touch access; a desktop screenshot alone is not sufficient.

Outside v1: JSON widget components, a built-in calendar, sound, advanced navigation/choreographed transitions, `lcars put`, server rollback history, live/animated/full-page capture, MCP, automatic skill installation, drag-and-drop layout, remote write APIs, upstream connectors, historical storage, and multi-user permissions. These are exclusions, not a backlog commitment.

## Scheduled scripts and immediate updates

Status: implemented with Linux race/browser tests and cross-compilation for Windows/macOS. Native Windows/macOS execution tests remain to be run on those systems. This extension adds an opt-in script runner and replaces normal browser polling with SSE. The existing data envelope and `LCARS.onData()` API stay unchanged.

### 1. Put the script beside its panel

```text
content/overview/
  weather.html
  weather.5m.py
  assets/weather/data.json   # published by LCARS
```

Use `<panel>.<interval>.<extension>`, parsed from the right. Intervals are positive whole numbers with `s`, `m`, `h`, or `d` (24 hours); reject overflow. Require an existing HTML panel and exactly one producer per panel. Only discover regular, non-symlink files directly inside section directories. Ignore dot-prefixed files; `.weather.5m.py` is disabled. Recognise scripts separately from panels and never serve their source. Report malformed scheduled filenames and duplicate producers during validation.

Enable execution only with `lcars serve --data ./content --run-scripts`. Ordinary serving and validation never execute scripts. Scripts run with the service account's permissions; this is trusted local code, not a sandbox.

### 2. Run, validate, publish

- Map `.py`, `.js`, `.sh`, and `.ps1` to Python, Node, a POSIX shell, and PowerShell. Allow installation-wide interpreter path overrides. Validate runtime availability; do not install runtimes. Shell scripts require a shell on Windows too.
- Invoke the interpreter with an argument array and the script path, using the section directory as the working directory. Close stdin. Pass a deliberate environment, including explicitly configured credentials; never publish credentials or raw logs to panels.
- Require stdout to contain exactly one UTF-8 JSON document using the existing version-1 data envelope. Allow trailing whitespace. Send diagnostics to stderr.
- Publish only after successful exit and envelope validation. Bound stdout by the existing data-file limit and retain only a bounded stderr log. Timeout, nonzero exit, invalid output, and publication failure retain the last good data.
- LCARS safely replaces `assets/<panel>/data.json`, then immediately scans the content store. Use platform-specific replacement where needed; never delete the old file first. Keep observation timestamps supplied by the producer. Identical bytes do not trigger a data update.

### 3. Keep scheduling small

Run once on startup, then wait the filename interval after completion. Never overlap a job or replay missed intervals after sleep. Retry failures at the next interval. Start with a 30-second timeout and a global limit of four running jobs; allow global overrides. Enforce one runner per content root.

Reuse periodic scanning to discover additions, edits, renames, and removals. Cancel obsolete invocations and reject their results before publication. Use process groups on Linux/macOS and Job Objects on Windows for subprocess cleanup, including timeout and shutdown; bound pipe draining too.

Record last attempt, last success, duration, and failure separately from data freshness. Surface a concise failure through existing panel diagnostics; keep detailed logs local. No per-job manifests, cron syntax, dependency installation, or retry queues.

### 4. Notify browsers with SSE

Add a private, authenticated-by-the-existing-proxy `/api/events` endpoint. Send only a state-change notification with the current revision. Use one `EventSource` per browser tab; the shell fetches `/api/state` and delivers changes through the existing panel bridge.

Subscribe before sending the initial revision. Fetch state whenever the connection opens or reconnects; no event history is needed. Coalesce notifications, serialize state fetches, and fetch again if a notification arrived during a request. Send heartbeat comments, flush events, disable proxy buffering, and release subscribers on disconnect. Fall back to the existing polling loop after repeated connection failures, stopping it when SSE recovers.

Keep the one-second filesystem scan for external edits. Script publication triggers an immediate scan, so it does not wait for either polling interval. Update panels without reloading their frames or resetting interaction state.

### 5. Verify and document

Implement in order: discovery and validation → runner and publication → SSE → documentation. Test filename errors, duplicate producers, invalid/partial output, timeout with children, safe replacement, disabling during a run, and no overlapping execution. Test SSE connection/reconnection, changes during a fetch, fallback polling, and a live panel retaining interaction state. Exercise process cleanup and file replacement on actual Linux, macOS, and Windows runners.

Update the README and bundled authoring guide with one working script-and-panel example, runtime setup, the execution flag, and stdout/stderr rules. Done when editing or scheduling data updates an open panel promptly, failed runs preserve its data, and ordinary serving remains read-only.
