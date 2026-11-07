# LCARS: an agent-fed canvas

Status: design only. No application has been built. Commands, paths inside the proposed data directory, and API shapes below are the intended implementation contract.

## 1. Product decision

Build one Go binary that serves a calm, useful, TNG-inspired web interface. A person's own agents collect information, write content, and generate visuals. LCARS provides navigation, layout, reusable components, live refresh, and a reliable screenshot loop. It contains no model, prompts, agent runtime, credentials for upstream services, or automatic interpretation of content.

The shortest useful workflow is: create a tab folder, write a Markdown file inside it, see a text panel on screen. Structured panels use JSON; a custom diagram adds one SVG file. Nothing requires a frontend build, account, plugin, or bespoke SDK.

Initial uses: an email briefing, a notification feed, home status, and telemetry. This is a read-only information canvas: following a source link is supported; sending mail, dismissing upstream notifications, and controlling appliances belong to the user's other tools.

## 2. Visual research and direction

Research reviewed on 7 November 2026:

- [TNG, “The Hunted”: screen reference](https://www.ex-astris-scientia.org/observations/thehunted/09-thehunted-r.jpg), with [episode context](https://www.ex-astris-scientia.org/observations/thehunted.htm). The inspected image has broad pale elbows, black separating gaps, rounded control ends, tightly condensed labels, and large black working areas. Borrow the proportions and hierarchy, not the camera distortion or tiny lettering.
- [Vessel-status screen reference](https://gtjlcars.de/images/LCARS-Entstehung/twisted246.jpg), from [LCARS history](https://gtjlcars.de/LCARSindex/Entstehung.htm). The inspected image separates a large technical schematic from dense supporting readouts using coloured rails. Use that relationship for home diagrams and telemetry. This is a broader franchise reference, not evidence of one exact TNG palette.
- [Mike and Denise Okuda interview](https://trektoday.com/interviews/okuda_qa.shtml). Their explanation emphasizes organization and the difference between convincing screen graphics and usable software. Practical consequence: actual labels, meaningful values, predictable interactions, and readable prose.
- [TheLCARS.com](https://www.thelcars.com/) demonstrates a responsive HTML/CSS interpretation. Treat it as secondary inspiration, not a dependency or a source of code to copy.
- [Antonio font licence](https://raw.githubusercontent.com/google/fonts/main/ofl/antonio/OFL.txt): SIL OFL. Bundle Antonio and its licence for headings and navigation. It is our practical condensed-font choice, not a claim of exact screen typography.

The result should feel like a working Enterprise terminal: black canvas, continuous asymmetric framing, warm apricot/orange structure, lilac secondary structure, occasional blue data, and restrained animation. Avoid generic rounded dashboard cards with an LCARS colour wash. Avoid gradients, glass, shadows, glowing text, starfield wallpapers, decorative fake telemetry, and illegible serial numbers.

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

## 3. Layout owned by the application

### Shell and components

- `Shell`: black canvas, skip link, labelled navigation landmark, connected top elbow/header and left navigation rail, main region, small connection/last-update footer.
- `NavItem`: broad segmented coloured strip with rounded end, readable section name and optional real item count. Active item has a marker and `aria-current`, not only a colour change. Decorative rail segments are not buttons.
- `PanelFrame`: titled open-centre frame with left spine, curved upper/lower joins, and short horizontal end caps. All panels share this construction; content never draws its own duplicate frame.
- `StatusLabel`, `Timestamp`, `EmptyState`, and `SourceLink`: consistent semantics and presentation.
- Content primitives: text, item list, metrics, table, chart, image. Panels can contain several blocks, stacked in supplied order.

At desktop widths the shell supplies the bold LCARS silhouette; panel frames are thinner so data remains dominant. Navigation stays available while the document scrolls. On short screens the rail itself may scroll. No mandatory fullscreen or fixed-height viewport.

### Deterministic placement

Each immediate subdirectory of `content/` is a tab (also called a section), selected through the left navigation at `/s/{folder-name}`. Creating a folder creates a tab. The folder is the source of truth: no tab registry and no `section` field in a panel.

Folder names supply labels by replacing hyphens with spaces. An optional `_section.json` inside a tab overrides `title`, `order` (default 100), and `accent` (default lilac); it never lists panels. Sort tabs by order, then folder name. An optional root `_canvas.json` contains only `version: 1` and a site `title`. Neither file is needed for a working canvas.

Example optional `home/_section.json`:

```json
{"version": 1, "title": "Home systems", "order": 20, "accent": "apricot"}
```

Only one level of tab folders is recognized in v1. Inside a tab, subdirectories other than `assets/` are ignored with a validation warning. Within a tab, ordinary `.md`, `.txt`, and `.json` files become panels; `assets/` holds referenced visuals. Ignore dotfiles, temporary files, and unsupported extensions; report unsupported visible files as validation warnings. Reserve underscore-prefixed files for application metadata. Tabs with no panels show an empty state. Removing the current tab navigates to the first remaining tab; no tabs shows the canvas welcome screen.

The content area uses a CSS grid based on its own available width:

- Under 720px: one column.
- 720–1199px: two equal columns.
- 1200px and above: three equal columns.
- `size: "normal"` occupies one column; `"wide"` occupies up to two; `"full"` occupies all columns. Default is normal. Clamp spans to the current column count.
- Sort panels by `order` (default 100), then stable panel ID. Use ordinary row placement, not dense packing or masonry. Preserve DOM/reading order and accept occasional empty space.
- Let row height follow content. Frames stretch within a row; their contents stay top-aligned. No automatic shrinking, cropping, or pixel-coordinate placement.

Below a 760px viewport, replace the full rail with a compact header and keyboard-accessible section menu. Keep one elbow as the visual signature. At every size, titles wrap, long URLs break, tables may scroll inside their own region, and the page never acquires accidental horizontal scrolling. A panel's `min-width` must be zero.

Default lists show up to 8 entries with an explicit “Show all (N)” disclosure. Default tables show 12 rows with the same mechanism. Keep expanded state, focus, and scroll position across updates. Text remains fully available. Do not silently discard overflow or turn every panel into a nested scroller. Move oversized material into a dedicated section rather than reducing type size.

## 4. Content contract: files first

The filesystem is the content model. Independent panel files let agents publish without rewriting a shared document:

```text
content/
  _canvas.json                   # optional site title
  overview/                      # tab, discovered automatically
    morning-briefing.md           # text panel, no metadata required
    notifications.json           # structured panel
  home/                          # another tab
    _section.json                # optional tab title/order/accent
    climate.json                 # structured panel
    notes.txt                    # plain text panel
    assets/
      climate/
        floorplan-v3.svg
```

A tab ID is its folder name; a panel ID is its filename stem. Both use lowercase letters, digits, and hyphens, 1–64 characters. Full identity is `tab/panel`, such as `home/climate`. IDs are unique within a tab: `briefing.md` and `briefing.json` conflict, rather than becoming two panels. Labels default to humanized filenames. Changing the extension preserves identity; moving to another tab or renaming the stem creates a new identity and resets local disclosure state.

### Text with no ceremony

`overview/morning-briefing.md` can be just:

```markdown
# Morning briefing

Two emails need a reply. The house is comfortable.

- Reply to Alex about Thursday.
- Review the maintenance notice.
```

If the first Markdown block is a level-one heading, use it as the panel title and omit it from the body; otherwise use the filename. Render paragraphs, headings, lists, emphasis, links, and code with a maintained Go Markdown parser. Disable raw HTML and remote images; local Markdown images follow exactly the JSON image rules, including required alt text. Normalize body headings beneath the panel heading. `.txt` renders literal text with preserved line breaks and a filename-derived title.

Do not add YAML front matter, paired metadata sidecars, or filename ordering conventions in v1. Text panels use default layout and freshness-unknown state. If a panel needs explicit order, width, observation time, or structured data, use JSON. A JSON `markdown` block can keep rich prose while enabling those controls; it uses the same parser as `.md` files.

### Structured content

Each panel JSON has `version: 1` and `blocks`. `title` defaults to the filename. Optional fields: `order` (100), `size` (normal), `accent` (inherited from tab), `source` (plain label), `updatedAt` (RFC3339), and `staleAfterSeconds` (positive integer). Accents accept only the named palette colours; statuses accept `normal`, `warning`, `critical`, or `unknown`. No arbitrary CSS, JavaScript, templates, or embedded HTML in content.

Example: a complete minimal panel, saved as `overview/morning-briefing.json` (instead of the Markdown version):

```json
{
  "version": 1,
  "title": "Morning briefing",
  "blocks": [
    {"type": "text", "text": "Two emails need a reply. The house is comfortable."}
  ]
}
```

Example: `home/climate.json`:

```json
{
  "version": 1,
  "title": "Home climate",
  "order": 20,
  "size": "wide",
  "accent": "apricot",
  "source": "Home Assistant",
  "updatedAt": "2026-11-07T09:00:00Z",
  "staleAfterSeconds": 300,
  "blocks": [
    {"type": "metrics", "items": [
      {"label": "Living room", "value": 21.4, "unit": "°C", "status": "normal"},
      {"label": "Power", "value": 420, "unit": "W"}
    ]},
    {"type": "image", "src": "assets/climate/floorplan-v3.svg", "alt": "Living room 21.4°C; bedroom 19.8°C.", "caption": "Latest room temperatures"}
  ]
}
```

### Seven block types

| Type | Required content | Optional content / behaviour |
|---|---|---|
| `text` | `text` string | Plain text with line breaks |
| `markdown` | `text` string | Same safe Markdown subset as standalone `.md` files |
| `list` | `items` array, each with stable `id` and `title` | Per item: `summary`, `timestamp`, `status`, `url`; email and notification rendering share this component |
| `metrics` | `items` array, each with `label` and `value` (number, string, or null) | `unit`, `status`; null renders “Unavailable”, never zero |
| `table` | `columns` array of `{key,label}`, `rows` array of objects keyed by column | String/number/null cells; no embedded markup; retain supplied row order |
| `chart` | `kind`, `series`, and axis labels/units as appropriate | Contract below; built-in SVG rendering and an accessible data disclosure |
| `image` | tab-relative `src` path within `assets/` and descriptive `alt` | `caption`; SVG, PNG, WebP, JPEG; preserve aspect ratio with contain, never crop |

Chart v1 supports only `line` and `bar`. A line has `xLabel`, `yLabel`, `unit`, and series of `{label, points: [[RFC3339 timestamp, number-or-null], ...]}`. Timestamps must be ascending; nulls create gaps. Bars have `unit` and series of `{label, points: [[category, number-or-null], ...]}`; categories must agree across series. Bars start at zero; line domains include finite data with padding and explicit ticks. Show a single point as a dot; empty data gets an empty state. No smoothing that invents values. At most four series, 500 points per series. Larger histories are aggregated by the agent. This is a snapshot canvas, not a time-series database.

Ship `schema/panel.schema.json`, `schema/section.schema.json`, `schema/canvas.schema.json`, and tested examples. Go validation is authoritative; schema/example agreement is tested. Reject unknown fields and block types with paths such as `blocks[1].items[0].value`, rather than silently accepting typos. Limits: 256 KiB per panel document, 20 blocks, 100 list items, 100 table rows, 10 MiB per image, 200 panels, and 100 MiB of unique accepted assets in total. Reject revisions that would exceed the aggregate budget while retaining the prior accepted state. State limits in validation errors; do not silently truncate input.

### Publish and update semantics

`lcars validate --data ./content` checks all documents, references, allowed asset types, and limits without starting the server. It prints actionable errors and returns nonzero on invalid input.

Ordinary file operations are the primary publishing interface. Write to a hidden temporary file on the same filesystem, then rename it over the target. For images, create a new filename in `assets/<panel-id>/`, then atomically replace the referencing document. Its rename is the commit point. Use relative references so moving a tab folder carries all its content with it. Moving an individual image-bearing panel between tabs also requires moving its asset directory.

Provide an optional convenience command: `lcars put home/climate --data ./content --file draft.json`. Infer format from `.md`, `.txt`, or `.json`; validate candidate content and assets, create the tab directory if needed, then atomically replace the target document. Print the document hash. Refuse a conflicting sibling extension with an instruction to remove it; never silently delete another format. Direct writers need no CLI. For example:

```sh
mkdir -p content/overview
printf '%s\n' '# Briefing' '' 'Two emails need a reply.' > content/overview/.briefing.md.tmp
mv content/overview/.briefing.md.tmp content/overview/briefing.md
``` No custom write API in v1; remote agents use their existing SSH/filesystem access.

One writer owns each full panel identity and its asset subdirectory; `put` does not merge edits. Different agents use different identities. Same-path atomic replacements are last-writer-wins. Shared ownership requires external coordination. Asset directories do not create panels; copying an SVG there alone does not publish it. Reference it from Markdown or JSON to supply its title and alternative text.

The server scans the small content tree every second, validates changed content, and caches accepted panels and referenced asset bytes in memory. Hash file bytes rather than relying only on modification time. Cache immutable assets by content hash; replacing a file cannot mutate an already accepted revision. Retain superseded assets for a 60-second browser grace period, then evict unreferenced bytes; bound the grace cache separately to 100 MiB, evicting oldest entries first. Build and publish the next view once per scan so readers do not see a half-processed directory listing. Each valid panel can advance independently of another panel's failure. No general multi-file transaction system.

Invalid edits retain the previous valid panel with an “Update rejected” badge and diagnostics. On cold start show a failed-panel placeholder. Duplicate stems invalidate that identity; retain the prior version if available. Missing assets invalidate the referencing revision. Invalid optional metadata retains its previous settings, or uses defaults on cold start, with visible diagnostics.

Deleting a panel file removes it on the next successful scan; deleting a tab removes its panels and navigation entry. A tab rename acts as removal plus discovery. An unreadable directory is an ingestion failure, not deletion: retain its previous content and report the error. Require the content root to exist at startup; subsequent root loss retains the last view with an error. Do not serve arbitrary files from the data root. Users can keep the directory in Git or back it up with ordinary tools; LCARS never commits on their behalf.

`updatedAt` is the producer's observation time; file scan time is not data freshness. `staleAfterSeconds` requires `updatedAt`; a stale panel remains visible with its age and “Stale” label. Missing timestamps mean freshness is unknown. Connection state, ingestion errors, source status, and staleness are distinct. No automatic expiry or disappearance of important messages.

## 5. Ad-hoc visuals: agent instructions to ship

Provide a short, provider-neutral `docs/AGENT_GUIDE.md`, also served read-only at `/agent-guide`, with examples available at `/examples/` and schemas at `/schema/`. Do not require users to install a particular agent skill.

The guide should give this decision order:

1. Create a tab folder and write `.md` or `.txt` for prose. Use JSON blocks for messages, values, tables, or a simple line/bar chart. Send content, not styling instructions.
2. For a floor plan, flow diagram, timeline, network map, or unusual plot, generate a standalone SVG. The agent chooses its own generation tools.
3. Use PNG/WebP for photos or complex raster output. Avoid rasterizing text-heavy diagrams when SVG is possible.
4. Save the image under `assets/<panel-id>/` in its tab with a new filename, publish Markdown or JSON referencing it, then screenshot the actual page and inspect it.

SVG guidance: use a `viewBox` (start with `0 0 800 450`), transparent or black background, generous margins, flat palette colours, and strokes at least 2 source units wide. Aim for labels at least 16 CSS pixels at the final displayed width. Put important meaning in the panel caption/alt as well. The host supplies the frame and title. Do not draw navigation or an outer LCARS panel inside the image.

Images do not inherit the parent CSS or fonts. Supply a downloadable palette JSON and SVG starter. Use literal palette values and a system sans-serif stack inside SVG; do not rely on external fonts. Keep important labels in native blocks if they need precise typography or accessible structured data.

SVG is static in v1: prohibit scripts, event handlers, foreignObject, external references, embedded resources, and animation elements/styles. Validate a documented SVG element/attribute allowlist; render through `<img>`, never inject agent SVG into the page DOM. Sanitization must fail visibly rather than quietly removing data. Do not accept arbitrary HTML or iframes; native charts provide the initial interactive data disclosure. Revisit sandboxed custom HTML only if real use cases cannot be served by these formats.

Starter visual:

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 450">
  <rect x="40" y="40" width="720" height="370" rx="24"
        fill="none" stroke="#CC99CC" stroke-width="4"/>
  <path d="M400 40V410" stroke="#CC99CC" stroke-width="3"/>
  <g fill="#F5EEE6" font-family="sans-serif" font-size="30">
    <text x="70" y="105">LIVING ROOM</text>
    <text x="70" y="160">21.4°C</text>
    <text x="430" y="105">BEDROOM</text>
    <text x="430" y="160">19.8°C</text>
  </g>
</svg>
```

Agent workflow: read the guide → choose your tab folder and panel filename → gather and summarize information → write content and any assets → validate/publish → capture desktop and narrow views → open both images → fix wrapping, density, labels, or ordering → repeat until the content is clear. Never invent telemetry to fill empty space. Use source links and observation timestamps where useful.

## 6. Screenshot iteration is a first-class feature

Implement `lcars screenshot` in Go using [chromedp](https://github.com/chromedp/chromedp), with an installed Chrome/Chromium binary. The renderer/server needs no browser; this command does. Support `--browser` and report a clear installation hint if absent. No Node runtime or browser automation service required.

Proposed usage (the address below is for internal local diagnostics):

```sh
lcars serve --data ./content --listen 127.0.0.1:8080
lcars put home/climate --data ./content --file ./draft.json
lcars screenshot --url http://127.0.0.1:8080/s/home --width 1440 --height 900 --out overview.png
lcars screenshot --url http://127.0.0.1:8080/s/home --width 390 --height 844 --out narrow.png
lcars screenshot --url http://127.0.0.1:8080/s/home --panel home/climate --out climate.png
```

Default capture is the viewport; `--full-page` captures all vertical content; `--panel` captures a frame via its stable `data-panel-id`. Provide `--expect-panel-hash` using the `put` result, so agents cannot accidentally inspect an older revision. A missing panel or unmatched hash times out with an error.

When an expected hash is supplied, wait for it before selecting the snapshot. Capture mode then loads that one accepted snapshot, freezes polling and clock-relative labels, disables transitions/animations, waits for `document.fonts.ready`, image decoding, and native chart completion, then sets `window.__LCARS_READY__`. Settle layout for two animation frames. Use a fixed device scale of 1. `--at` fixes the display clock for fixture comparisons; it does not alter source timestamps. Fail after 15 seconds with diagnostics, not a misleading success image.

Save a neighbouring JSON report with viewport, document revision, panel hashes, browser version, console/page errors, failed assets, and page-level horizontal overflow. Nonzero exit on these rendering failures. An agent must still inspect the PNG for readability and composition; a machine check cannot judge those fully. The same readiness hook supports whichever browser tools the user's agent already has.

Screenshots run locally as an explicit CLI operation. Do not expose a server endpoint that launches browsers at arbitrary supplied URLs. A screenshot of the real feed may contain private information; write artifacts to the requested path, not a public gallery.

## 7. Motion and interaction

Default motion is quiet and sparse:

- One small decorative segment in the shell softly varies opacity from 0.85 to 1 over 8 seconds; no moving text or full-screen sweep.
- Accepted changed content gets one 200ms frame accent transition. Do not animate unchanged panels or count numbers through invented intermediate values.
- Hover/focus colour transitions last 120ms. No boot sequence, audio, auto-scrolling, flashing warning banners, or continuous chart animation.
- Stop decorative animation while the tab is hidden. Support `prefers-reduced-motion` and a persistent user “Motion off” toggle; either disables all optional motion. Screenshot mode does likewise.

Use semantic links/buttons, visible focus rings, touch targets at least 44px high, and appropriate heading hierarchy. Do not announce every telemetry tick to screen readers. Announce connection loss/recovery politely; offer timestamps and data disclosures for deliberate inspection. Aim for WCAG AA text contrast and verify actual rendered combinations.

## 8. Go implementation and runtime

Keep the project small:

```text
cmd/lcars/            # serve, validate, put, screenshot
internal/content/     # parsing, validation, scan/cache, atomic writes
internal/server/      # routes and accepted snapshot projection
internal/capture/     # chromedp screenshot command
web/                  # embedded HTML, CSS, small vanilla JS, local fonts
schema/               # versioned content contracts
examples/             # complete example content and SVG starter
docs/                 # agent guide and operational notes
```

Use `net/http`, `encoding/json`, `embed`, and ordinary files. Use CSS Grid for layout and small JavaScript renderers using safe DOM methods; do not put JSON values into `innerHTML`. Keep dependencies to browser capture, a maintained Markdown parser, and any justified schema-validation library. Markdown output must use a safe renderer with raw HTML disabled and validated link/image destinations; do not trust arbitrary generated HTML. No frontend framework, bundler, database, WebSocket, queue, or plugin system.

HTTP surface: `/` redirects to the first section; `/s/{section}` serves the shell; `GET /api/state` provides accepted content, diagnostics, and revision with ETag; `/assets/{hash}/{name}` serves accepted image bytes; `/healthz` indicates process health. The browser conditionally polls state every 2 seconds while visible, pauses when hidden, and refreshes immediately on return. Patch only changed panels, retaining focus and disclosure state. A failed fetch keeps the last display with a disconnected label; retry with backoff up to 30 seconds. Expected normal update visibility is within about 3 seconds plus rendering/network time.

Bind to loopback by default. Support deployment behind the user's private authenticated proxy; this app has no public multi-user authentication model. No upstream fetching: an image URL cannot cause server-side network access. Allow source links only with `https:` or `http:` schemes, and safe new-tab behaviour. Escape all text. Refuse path traversal and symlinks in the content tree, enforce type/size limits, use CSP and `nosniff`, and serve SVG with a restrictive asset policy as defence in depth.

When deployed on a tailnet machine, discover its full hostname with `tailscale status --json`, preserve the actual service port/path, and verify reachability before sharing links. On Kitt, also update the private homepage registry in `/home/matryer/code/github.com/matryer/devtools` following its repository instructions. Planning this application does not require launching or registering a service.

## 9. Build order and acceptance

1. **Vertical slice:** Go server, folder-discovered tabs, shell/nav/frame, Markdown/text panel, file refresh, empty/error states, and screenshot command. Inspect 1440×900 and 390×844 captures against the reference principles before adding components.
2. **Content toolkit:** JSON blocks, lists, metrics, tables, native charts, image validation/rendering, timestamps, and staleness. Add complete examples for email, notifications, home climate, and telemetry.
3. **Agent workflow:** schemas, `validate`, atomic `put`, served guide, SVG starter, screenshot hashes/reports. Follow the guide from an empty directory without unpublished knowledge.
4. **Hardening:** responsive and keyboard checks, motion preferences, long/empty/malformed content, deletion/restart behaviour, concurrent independent publishers, and deployment documentation.

Release gates:

- A new agent can publish its first text panel by creating a folder and a Markdown file, with no metadata or app code changes.
- Moving/deleting files and folders updates panels/tabs predictably; invalid and unreadable files do not masquerade as deletions. A second agent can publish a new panel without modifying the first; custom SVG requires no registration or rebuild.
- Four representative use cases render coherently at 1920×1080, 1440×900, 1024×768, and 390×844, and remain usable at 200% zoom.
- Long titles, long email subjects, Unicode, missing values, empty datasets, and large tables have explicit readable behaviour. No page-level horizontal overflow.
- A bad edit or missing image cannot blank a previously working panel. Stale data is never presented as freshly observed; disconnects do not erase content.
- Screenshot capture reliably waits for the requested content revision and produces inspectable PNGs; fixed fixtures produce stable captures on the same browser/platform.
- Normal motion remains unobtrusive during a 60-second observation; reduced-motion and Motion off produce no optional animation.
- Unit tests cover validation, path/asset safety, ordering, freshness, atomic publication, and last-good retention. Browser checks cover navigation, updating without focus loss, breakpoints, screenshot readiness, and contrast. No upstream integration is needed for tests.

## 10. Design review decisions

The plan has been reviewed against simplicity, agent effort, visual fidelity, and failure recovery:

- **No AI layout:** fixed grid rules are predictable, cheap, and screenshot-testable.
- **Files before APIs:** one canonical content path works with ordinary agents and SSH. An HTTP write API can wait for an actual remote publishing need.
- **Folders are tabs, files are panels:** navigation is discovered and panel ownership follows paths. Optional metadata only changes presentation; no manifest duplicates the directory tree.
- **Markdown before structured content:** ordinary prose needs only a file; JSON adds layout and data controls when useful.
- **Native blocks before custom visuals:** common data stays accessible and automatically themed; SVG covers unusual visuals without a plugin runtime.
- **Static SVG before executable embeds:** retain a manageable rendering and security boundary. Rich bespoke interactions are deferred explicitly.
- **Polling before streaming:** small snapshots and modest update frequency fit this canvas; high-rate telemetry must be summarized upstream.
- **Readable content before cinematic decoration:** shell geometry establishes LCARS, leaving room for real information.
- **Screenshot tooling in the first slice:** visual quality is part of the development loop, not a final-stage task.

Defer drag-and-drop layout, arbitrary themes, remote write/auth APIs, interactive custom HTML, upstream connectors, historical storage, notification actions, and multi-user permissions. The implementation should prove this small contract before expanding it. Pixel proportions and animation comfort remain visual-validation tasks; a written plan cannot establish those by itself.
