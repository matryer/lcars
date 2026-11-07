# Authoring LCARS panels

Authoring contract **1.0**; bridge and JSON schemas use version 1. Discover the app build with `lcars version --json` or the private `/api/version` endpoint. Read this guide with `lcars skill --guide`, print a starter with `lcars skill --starter`, or export the entire portable skill to a new directory with `lcars skill --out ./lcars-authoring`. This is the canonical guide, also served at `/agent-guide`. LCARS provides navigation, frames, shared styles, update times, isolated HTML, live data, and capture tooling. Authors supply ordinary files. An opt-in local scheduler runs adjacent scripts; there is no widget language, built-in upstream connector, or agent runtime.

## Plan the information architecture

Decide what belongs at which level before writing any panel; it shapes everything else. A canvas is a drill-down: the highest-level information sits on the first levels, and the viewer clicks in for more.

| Level | Holds | Example |
| --- | --- | --- |
| Section (rail) | An overview of one domain | Systems |
| Sub-section (tab) | A distinct view within it | Systems · Storage |
| Panel | A summary of one topic, leading with the headline answer: a status, count, next item or total, not a raw list | Volumes: “3 of 4 normal”, then a bar per volume |
| Detail view (`LCARS.openDetail`, with Back) | The specifics of one item | vault · tank: used, free, size, last scrub |
| Link (`LCARS.navigate`) | A jump from a summary to the page that owns the detail | Overview's "Needs attention" item opens Systems · Storage |

Rules:

- Each level answers "what needs my attention, and what's next?" before it shows breadth.
- Summarise, then let the viewer drill: counts and states first, items on click.
- Keep the path short: overview → page → panel → detail at most.
- Make every summary item that has more behind it clickable, with a clear affordance (a button styled as a row, a dotted underline, an arrow), and leave items with nothing behind them as plain text.
- Do not duplicate full detail at higher levels. The overview names the problem; the owning page explains it.
- Keep cross-links consistent: an item always opens the same place, from wherever it appears.

A worked example: Overview's *Needs attention* panel says "Offsite backup failed overnight · Open Systems · Storage →". Pressing it opens Systems · Storage, whose *Backups* panel leads with “2 of 4 normal” and then each job's state; pressing the failed job opens its detail (the error and last success time) in that panel, and Back returns to the list. *Volumes* on the same page works the same way: the bar shows how full each volume is, and pressing a volume opens its used, free and scrub details.

## Choose a path and format

Find the existing deployment's data root and URL; do not assume this repository is its live content directory. Each immediate folder is a section at `/s/{tab}`, listed in the rail. A folder inside a section is a sub-section at `/s/{tab}/{sub}`, shown as a tab bar under the section title; sub-sections are one level deep. Each direct `.html`, `.md`, or `.txt` file is a panel:

```text
content/
  overview/
    briefing.md
    energy.html
    assets/
      energy/
        data.json
        diagram.svg
```

A sub-section is laid out the same way, with its own optional `_section.json` and `assets/`:

```text
content/
  systems/
    _section.json        # title "Systems"
    hosts.html           # systems/hosts, on the first tab
    network/
      _section.json      # title "Network"
      latency.html       # systems/network/latency
      latency.5m.py
      assets/latency/data.json
```

A section's own panels appear on its first tab, named after the section; sub-sections follow in their order. A section with only sub-sections opens its first one. Use sub-sections when a topic has distinct views worth separate pages, not to nest a single panel.

Folder and file names are 1–64 lowercase letters, digits, or hyphens. Panel IDs join them with `/`: `overview/energy`, `systems/network/latency`. Duplicate filename stems across formats are invalid. Hidden and temporary files are ignored. Visible unsupported files/directories produce diagnostics. `assets/` never creates panels.

Own one panel document and its asset directory. Publish different panels independently; agree on ownership before overwriting another author's panel. Write only the information the user requested and preserve its original meaning. Source retrieval and upstream actions use the agent's existing tools and authorization. A request to display calendar entries grants no permission to edit them.

Choose Markdown for prose, plain text for preserved line breaks, and HTML for interactive layouts, observation metadata, diagrams, or custom presentations. A Markdown first H1 supplies the frame title and is omitted from its body. Paragraphs, headings, lists, emphasis, links, and code are supported; raw HTML and remote images are disabled. Markdown images need descriptive alt text and a local PNG, JPEG, WebP, or passive SVG under that panel's `assets/{id}/` directory.

HTML `<title>` supplies its frame title. Fragments also work. Optional metadata:

```html
<title>Household energy</title>
<meta name="lcars:size" content="wide">
<meta name="lcars:order" content="20">
<meta name="lcars:accent" content="blue">
<meta name="lcars:updatedAt" content="2026-11-07T09:00:00Z">
<meta name="lcars:staleAfterSeconds" content="300">
<meta name="lcars:sourceUrl" content="https://example.com/source">
```

`size` is `normal`, `wide`, or `full`; it is a maximum span, never a guaranteed pixel width. `order` is an integer, default 100. Accent names are `apricot`, `orange`, `lilac`, `blue`, `peach`, or `critical`; default lilac. `updatedAt` is the observation time in RFC3339, independent of publication time. `staleAfterSeconds` must be positive and requires `updatedAt`. `sourceUrl` accepts HTTP(S) and is rendered by the host as a safe source link. Omit optional metadata rather than supplying invented values.

An optional `_section.json` controls the tab:

```json
{"version":1,"title":"Overview","order":10,"accent":"apricot","theme":"pastel"}
```

Only these fields are accepted, and the same file works in a sub-section folder. `theme` is optional: `classic`, `pastel`, `nebula`, `tidal`, `sand`, or `meridian`. Without it, each top-level section gets a stable theme chosen from its ID, and its sub-sections share it. Set one only when the user asks for a particular look. Sections and sub-sections sort by order then ID; panels sort by order then ID. No site configuration or per-panel sidecar is needed.

## Fit the allocated width

Copy the bundled `assets/starter.html` relative to this skill folder. LCARS injects the base stylesheet and `window.LCARS` before author scripts, and wraps content in `.lcars-content`. It draws the external frame; do not duplicate its elbow and title around your interior.

Start with one column. The iframe viewport is the panel's actual width, so its media queries naturally respond to allocation. Desktop pages can still allocate a narrow column. Use shrinkable CSS Grid tracks (`minmax(0, 1fr)`), allow Flexbox children to shrink, and wrap controls. Convenience classes `.lcars-row` and `.lcars-stack` provide wrapping flex and a single-column grid. The toolkit applies border-box sizing, zero body margin, fluid images/SVG/Canvas, and long-word wrapping.

Avoid fixed page widths, viewport-based heights, absolute positioning for the main layout, clipped focus, and transform scaling. The host measures intrinsic content height, beginning at 320px and clamping it to 80–2400px. Taller panels retain an internal scroller with a diagnostic. Use a sensible compact layout to avoid height/width feedback loops, and make the first rendered height the final one (see “Fit page transitions”).

SVG diagrams need a `viewBox`, intentional aspect ratio, and accessible description. Draw original geometry. Canvas charts must resize their drawing buffer and redraw whenever their allocated width changes (a `ResizeObserver`, plus a redraw when the panel becomes visible), not once at load. Reflow legends, reduce tick density, and offer an accessible data disclosure. Do not shrink a large desktop bitmap until its labels are unreadable.

A useful brief might be: “Show my next seven days, highlight today, make overlaps obvious, and distinguish all-day events in my local timezone.” Choose a calendar grid only when it fits; use an agenda on narrow panels. Keep the same data and selection state across reflow. Controls and details must work with touch and keyboard, without hover. Essential dates, times, and actions stay visible.

For genuinely two-dimensional tables, use a labelled, keyboard-accessible local horizontal scroll region with a visible scroll affordance. Code blocks may also scroll locally. Ordinary prose and controls must reflow; global overflow hiding is not a repair.

## Use the visual contract

The canvas is black. Use warm apricot/orange for primary structure, lilac for supporting structure, and blue/peach for data. The toolkit supplies CSS palette variables `--lcars-background`, `--lcars-text`, `--lcars-muted`, `--lcars-apricot`, `--lcars-orange`, `--lcars-lilac`, `--lcars-blue`, `--lcars-peach`, `--lcars-critical`, and `--lcars-ink`. Use these rather than hex values: the page theme recolours apricot, lilac, blue and peach (inside your panel too), while orange and critical stay fixed in every theme because they carry warning and critical meaning. Hex fallbacks such as `var(--lcars-blue, #99f)` are fine. Canvas drawings that read colours should re-read them from `getComputedStyle` when they redraw.

Use locally bundled Antonio for short labels, headings, and key values through `--lcars-label-font`; use the system sans-serif body stack for prose. Body text begins at 16px with 1.5 line height; labels at 18px; headings around 24–32px; values around 36–56px. Use 8/16/24px spacing, readable muted text, and black text on pastel bands. Keep titles selectable. The host decorates the page itself: segmented bars with codes in the frame, and LCARS keypads, readouts, meters and bar stacks that fill grid cells your panels leave empty. Do not add decoration, filler numbers, codes or fake telemetry inside a panel; everything in a panel must be real information. Avoid gradients, glass, shadows, glow, starfields, or all-uppercase paragraphs. The one sanctioned effect is the toolkit's scanning element for visualisations (see “Give visualisations a scanning element”). To change how much filler appears, change panel sizes, not the content.

Keep the interior lean. The frame already draws the panel title in capitals and the section title above it, so do not repeat or restate either inside the panel. Skip eyebrows, kicker labels, taglines, and sub-headings; a value needs a label only when the title does not already say what it is. Add a sub-heading only to separate genuinely distinct groups in a long panel, and never as the first element. Markdown panels follow the same rule: the first H1 becomes the title, so avoid `##` headings unless the content really has separate parts. Do not add your own `Updated …`, timestamp, or "demo"/"fixture" captions; the frame shows the time, and the title can carry a qualifier when one is needed.

Write health states explicitly: “Normal”, “Warning”, “Critical”, or “Unavailable”. Colour alone is insufficient; accent colour identifies a section, not its health. Chart series need labels or dash patterns as well as colour. Images need descriptive alt text and charts need descriptions or data access. Keep visible focus and 44px touch targets. Panels are silent.

## Separate code from live data

Write optional live data to `assets/{panel-id}/data.json`:

```json
{
  "version": 1,
  "updatedAt": "2026-11-07T09:00:00Z",
  "staleAfterSeconds": 300,
  "data": {"watts": 420}
}
```

`version: 1` and `data` are required; the inner data can be arbitrary JSON, including null. Only `version`, `updatedAt`, `staleAfterSeconds`, and `data` are allowed in the envelope. Observation fields follow HTML metadata rules and take precedence while the data file exists. Do not replace missing values with zero.

```html
<output id="power">Unavailable</output>
<script>
LCARS.onData(({data, missing}) => {
  document.querySelector('#power').textContent = !missing && Number.isFinite(data?.watts)
    ? `${data.watts} W` : 'Unavailable';
});
</script>
```

`onData(callback)` replays the initial envelope after handshake and delivers updates without replacing the frame, preserving local interaction state. Delivery includes `missing`; a deleted file differs from a present envelope with `data: null`. It returns an unsubscribe function. Returned callback promises are awaited before the host acknowledges a displayed hash. Callbacks run serially; pending ordinary updates coalesce to the latest value. Keep them bounded and report unavailable states honestly. A throw or a promise that takes over 10 seconds marks that frame unverified; Retry initializes a fresh frame.

`onState(callback)` delivers `{visible, motion, capture, clock}`: visibility, motion preference, capture state, and the display clock as an RFC3339 string. A preloaded or parked panel gets `visible: false`. Stop optional animation while hidden or motion is off. Initial rendering must work without animation ticks or `requestAnimationFrame`. `deferReady()` called during initial execution returns a completion function for asynchronous setup; call it when setup finishes, without waiting for an animation frame. Fonts, images, DOM readiness, and initial data are already handled. `onCapture(callback)` lets a diagram draw a still using the frozen display clock; returned promises are awaited. Capture does not record animation or offer timeline controls.

Use local classic scripts and styles in the owned asset directory. No CDN, network fetching, dynamic imports, workers, nested frames, form submission, popup windows (use `LCARS.openDetail`), download, or top navigation. The sandbox intentionally omits same-origin access. Components see only their own data; do not reach into the shell or other frames. Keep useful HTTP(S) source links in Markdown or `lcars:sourceUrl` for host rendering.

Passive SVG resources permit these elements: `svg`, `g`, `path`, `rect`, `circle`, `ellipse`, `line`, `polyline`, `polygon`, `text`, `tspan`, `title`, `desc`, `defs`, `clipPath`, `mask`, `linearGradient`, `radialGradient`, and `stop`.

Allowed attributes are `xmlns`, `viewBox`, `width`, `height`, `x`, `y`, `x1`, `x2`, `y1`, `y2`, `cx`, `cy`, `r`, `rx`, `ry`, `d`, `points`, `fill`, `fill-opacity`, `fill-rule`, `stroke`, `stroke-width`, `stroke-opacity`, `stroke-linecap`, `stroke-linejoin`, `stroke-dasharray`, `opacity`, `transform`, `font-family`, `font-size`, `font-weight`, `text-anchor`, `dominant-baseline`, `id`, `class`, `role`, `aria-label`, `aria-labelledby`, `preserveAspectRatio`, `offset`, `stop-color`, `stop-opacity`, `gradientUnits`, `gradientTransform`, `clip-path`, and `mask`. `url(...)` references must target local `#ids`. Other elements/attributes, scripts, event handlers, animation, external references, directives, and active embedded content are rejected. Symlinks and paths outside the owned asset directory are refused.

## Show times and details

Write times with the toolkit's relative-time element instead of formatting your own "3 hours ago" text:

```html
<p>Last backup <lcars-time datetime="2026-11-07T02:14:00Z" label="Last backup"></lcars-time></p>
```

It renders a compact `now`, `5m`, `2h`, `3d` or `in 3d`, follows the display clock (so captures stay frozen and consistent with the frame), refreshes itself, and opens a detail view with the exact time in the viewer's timezone when pressed. `label` names the time in that detail view and for screen readers. An invalid or missing `datetime` shows `—`; never substitute an invented time. Set the attribute from `onData` when the time comes from live data. The frame's own update time works the same way, so do not repeat it inside the panel.

For the specifics of one item, open a detail view from a user action:

```js
row.id = 'vault'; // a stable id lets Back return focus here even if the list re-renders
row.addEventListener('click', () => LCARS.openDetail({
  title: 'vault',
  body: 'Storage host.\n\nA blank line starts a new paragraph.',
  fields: [{label: 'Disk', value: '86%'}, {label: 'Last backup', time: data.lastBackup}]
}));
```

The detail view replaces the panel's content inside its own frame; it is not a popup. The toolkit hides the content without destroying it (its DOM, local state, scroll positions and listeners are kept, and `onData` keeps updating it), then draws the detail: a Back button, the title, the paragraphs and a grid of fields. Keyboard focus moves to the title. The host mirrors it on the panel frame: the heading becomes a breadcrumb such as `TODAY › DENTIST`, and a Back control appears on the frame's top bar. Back (either button) or Escape restores the content and returns focus to the element that opened the detail (or, if that element was re-rendered, to the element now carrying its `id`). The panel's height follows the detail view automatically, so keep it reasonably short.

Content is text only: `title` (required, up to 200 characters), `body` (up to 4,000; blank lines separate paragraphs) and up to 40 `fields`, each a `label` with either a string/number `value` or an RFC3339 `time` (shown exact and relative). Markup is displayed literally. `openDetail` returns a promise that resolves `true` when the viewer goes back, or `false` if it was replaced or could not open (for example during capture). Opening another detail while one is open replaces it, with one Back to the content. No user gesture is required, but open details in response to the viewer, not on a timer. Captures always show the panel's content. `LCARS.openModal` is a contract-v1 alias of `openDetail`; use `openDetail` in new panels.

To send the viewer to another page, call `LCARS.navigate('systems/storage')` from a click or key press, for example on an item in a summary that belongs to that page. It takes a section or sub-section ID, resolves `true` if the host navigated, and `false` if it refused (no user action, an unknown section, or capture). Top-level links and `target` navigation remain unavailable inside panels.

## Make panels worth touching

Good panels respond to the viewer without pretending to control anything. Useful patterns:

- Open details for an item (`openDetail`) rather than cramming every figure into the panel.
- Filter or sort a list with a row of toggle buttons (`aria-pressed`), keeping the choice across live data updates.
- Toggle chart series from the legend, and let the viewer select a day or scrub along a time axis with pointer and arrow keys, describing the selection in text (`aria-live="polite"`).
- Link a summary to the page that owns the detail (`navigate`).

Every interaction needs a keyboard path and a 44px target. Local view state (selection, filters, sort, scroll position, open disclosures) must survive `onData` updates and page visits; the shell keeps visited pages alive, so the viewer returns to the panel as they left it. Never add buttons that appear to change something upstream unless the user asked for that capability and it really works; the host's decorative keypads already provide playful, do-nothing buttons outside panels.

The shell adds its own interactions: an Expand control widens a panel to the whole row in place (its frame and state are kept), number keys `1`–`9` open sections and `[` / `]` step through a section's tabs. Design panels to reflow well when expanded.

## Give visualisations a scanning element

Give every chart, meter or bar group the toolkit's scanning effect, so the canvas looks like it is being scanned. Add the `lcars-scan` class to a wrapper around the visual (not to `<svg>`, `<canvas>` or `<img>` themselves, which cannot host it):

```html
<div class="lcars-scan" style="--lcars-scan-color: var(--lcars-text)">
  <svg id="chart" viewBox="0 0 600 190" role="img" aria-labelledby="chart-desc">…</svg>
</div>
```

A translucent band with a bright leading edge sweeps across the element every few seconds. `lcars-scan-radial` is a radar-style sweep for a square or circular visual. Tune it with `--lcars-scan-color` (default blue), `--lcars-scan-duration` (default `7s`) and `--lcars-scan-delay`; scans in one panel run in step, so a column of bars reads as one scan line. The element clips its contents 8px outside its box, enough for focus rings, and the band never adds overflow. The toolkit runs it only while motion is on and the panel is visible, pauses it while a detail view is open, and shows nothing in captures; it uses transforms and opacity, needs no script, and never alters the data. For a chart drawn in script, a sweep may also be chart-specific (a cursor following the band), as long as it moves along real axes and does not imply readings that are not there.

## Fit page transitions

After the current page is displayed, the shell preloads other pages' panels in the background (sibling sub-tabs first, then other sections) within a budget of live panels, and keeps visited pages alive, releasing the least recently used beyond the budget. Inactive pages are *parked*: laid out offscreen at their real width, inert, and hidden from assistive technology. Browsers throttle parked frames: `requestAnimationFrame`, painting and often `ResizeObserver` callbacks pause, while scripts, timers, `onData` delivery and data acknowledgement still run. A parked panel receives `onState` with `visible: false`, then `visible: true` when its page is shown. A panel may load long before the viewer sees it, and may be shown, hidden and shown again without reloading.

On navigation the shell measures the old page's frames, swaps pages, and morphs ghost LCARS frames from the old positions, sizes and accents to the new panels', so panels appear to be reused and rearranged; the new content then fades in and titles type in. Authors do nothing to opt in. With reduced motion the swap is instant, and captures never animate. Write panels that survive both:

- Render meaningful content synchronously from the first `onData` delivery. Do not gate the first paint on `requestAnimationFrame`, animation ticks or timers, and do not await them inside an `onData` callback or deferred setup: a parked frame may get no frames until it is shown, so the panel would arrive blank or its acknowledgement would stall.
- Lay out and draw from the current size whenever it changes (a `ResizeObserver`, plus a redraw on `visible: true` to cover callbacks missed while parked), never once at load. A panel may first lay out while parked.
- Keep the final height stable and known at first render. Reserve space for parts filled later (fixed rows, `min-height`, `aspect-ratio`) so the morph lands on the right size and nothing jumps after the content fades in.
- Pause polling-style work, animations and heavy timers while `visible` is false; on `visible: true`, resume and catch up from the latest data and clock rather than replaying what was missed.
- Loaded is not arrived. Start any arrival behaviour on the transition to `visible: true`, and compute relative times from the display clock (`onState`'s `clock` or `<lcars-time>`), not load time.
- Add no entrance animations, fade-ins or typewriter effects; the shell already animates frames and fades content in. Keep panel-internal motion data-driven or the toolkit's `.lcars-scan`, and stop it when `motion` is false.
- Keep the panel's identity, `lcars:accent` and `lcars:size` stable across updates; they decide which frame morphs where. Update through `onData` rather than republishing the document, because a document change re-stages the panel in a fresh frame.
- An open detail view persists across visits too; the viewer returns to it with its breadcrumb and Back. Do not close it on `visible` changes.
- Expect local view state to persist across visits and make it sensible to return to: keep it valid when new data arrives (drop a selection whose item disappeared, clamp a scrubbed position) and never reset it on `visible` changes.

## Schedule an adjacent data script

For periodic retrieval, place `<panel>.<interval>.<extension>` beside its HTML panel:

```text
content/overview/
  weather.html
  weather.5m.py
  assets/weather/data.json   # LCARS owns this output
```

Use positive whole-number intervals with `s`, `m`, `h`, or `d`; `1d` means 24 hours. Only direct section files are scheduled, never files inside `assets`. A matching regular HTML panel is required. Duplicate producers for one panel are errors. Prefix a script with a dot to disable it. Ordinary helpers such as `helper.py` are not scheduled; reserve dotted script stems for the scheduling format. Scripts and helpers are not served to browsers.

Execution requires `lcars serve --data /path/to/content --run-scripts`. Do not assume a deployed server has enabled it. Scripts run with the service account's filesystem and network permissions, not the HTML sandbox. Retain existing source-access authorization; use retrieval scripts for the requested data, not unrelated upstream changes.

The extension selects an installed runtime: `.py` → `python3`, `.js` → `node`, `.sh` → `sh`, `.ps1` → `pwsh`. The service can override each executable with `--script-py`, `--script-js`, `--script-sh`, or `--script-ps1`. A shell script needs an installed POSIX shell even on Windows. Use Python or Node for portable producers and avoid platform-specific commands. No runtime or dependency is installed automatically.

The working directory is the section directory, stdin is closed, and no implicit shell parses the command. Scripts receive basic path/home/locale/temp environment variables; additional variables require the service's `--script-env NAME,OTHER_NAME` allowlist. Python stdout uses UTF-8. PowerShell scripts must emit UTF-8 JSON themselves. Never put credentials in stdout, panel files, or diagnostic logs.

Print exactly one version-1 data envelope to stdout, within 256 KiB. Pretty printing and trailing whitespace are allowed. Send diagnostics to stderr and exit nonzero on failure. Publication happens only after successful exit and validation. LCARS replaces the output file safely; the script must not write it directly. Preserve actual source observation times; a successful fetch does not make old source data new.

A working local example, labelled with its actual source:

```python
# machine.1m.py, beside machine.html
import json
import platform
from datetime import datetime, timezone

print(json.dumps({
    "version": 1,
    "updatedAt": datetime.now(timezone.utc).isoformat(),
    "staleAfterSeconds": 180,
    "data": {"hostname": platform.node(), "system": platform.system()}
}))
```

```html
<!-- machine.html -->
<title>Serving machine</title>
<p id="machine">Unavailable</p>
<script>
LCARS.onData(({data, missing}) => {
  document.querySelector('#machine').textContent = !missing && data
    ? `${data.hostname} · ${data.system}` : 'Unavailable';
});
</script>
```

Jobs run once at startup, then wait their interval after completion. Missed intervals are not replayed; runs never overlap for a panel. Defaults are a 30-second timeout and four concurrent jobs, overridden globally with `--script-timeout` and `--script-concurrency`. Changed, hidden, or removed scripts invalidate in-flight results. Failures keep the last accepted data and appear in panel diagnostics. Retry occurs at the next interval. The private state API records attempt/success times and duration separately from data observation time; bounded stderr stays in service logs.

Publish script edits using hidden temporary files, then rename into place. Validate filenames and envelopes with `lcars validate` (it never executes scripts or checks runtime availability). Check the enabled server's diagnostics to verify an actual run, then capture and inspect the resulting panel. Ordinary serving, validation, and snapshot capture remain non-executing. Do not enable execution or restart a deployment as part of ordinary authoring unless that is within the user's requested scope.

## Publish a consistent revision

Write hidden temporary files on the same filesystem and atomically rename them over their targets. For code/resources, stage the entire asset set first, then replace the document **with changed bytes** as the commit point. A changed comment is enough to publish resource-only changes. The app snapshots document plus all resources in `assets/{id}/`, except live `data.json` and hidden/temporary files. Accepted bundles are immutable; editing an asset alone does not alter the displayed bundle.

```sh
mkdir -p "$LCARS_DATA/overview"
printf '%s\n' '# Briefing' '' 'Two items need attention.' > "$LCARS_DATA/overview/.briefing.md.tmp"
mv "$LCARS_DATA/overview/.briefing.md.tmp" "$LCARS_DATA/overview/briefing.md"
lcars validate --data "$LCARS_DATA"
```

`LCARS_DATA` here is an author-supplied variable for the existing data root. Live `data.json` can be replaced atomically without committing the HTML again. Independent panels publish independently. External edits are scanned every second. Accepted state changes notify visible browsers through SSE; the shell fetches current state and delivers changes to `LCARS.onData()`. Script publication triggers a scan immediately. Browsers pause while hidden, fetch current state on reconnect, and fall back to polling when SSE is unavailable. Panels do not open their own SSE connections.

Invalid replacements retain the accepted panel/data with diagnostics. New invalid panels get a placeholder. Unreadable roots retain prior content; confirmed deletion removes a panel. Code replacement stages a new frame while the old frame remains visible, then commits after readiness and initial-data acknowledgment. It has a 10-second deadline. Failed code is tried once per page session until Retry or a newer revision. A failed data callback stops delivery until Retry; its old hash is last-verified evidence, not a claim about current DOM.

Limits: 200 panels; HTML 1 MiB; Markdown, text, and data envelopes 256 KiB each; individual resources 10 MiB; unique current resource bytes 100 MiB total. Validation checks limits before reading oversized files. Released bundles have a grace cache limited to 60 seconds and 100 MiB; it is not history or rollback. An old frame requesting an expired resource must retry the latest revision.

## Read publication and observation times correctly

Every frame shows one compact relative time (`now`, `5m`, `2h`, `3d`); pressing it shows a row of exact times and their source under the panel. The section heading shows the latest panel's time the same way. When the producer supplied an observation time, the frame shows that; otherwise it shows the publication time of the acknowledged displayed document/resources/data revision. Unchanged bytes, polling, pending candidates, and rejected edits do not advance publication time. File times seed the value at startup; they are publishing evidence, not an audit record.

Observation time comes only from the producer. If none was supplied it stays unknown; do not invent one. An elapsed stale threshold adds a visible `Stale` label. Updating a title does not freshen old measurements. A missing live file shows data unavailable. Connection status and last successful check in the shell footer do not describe content freshness.

## Validate, capture, inspect, revise

For a directory-wide check, `lcars snapshot --data "$LCARS_DATA" --out ./captures` validates first, starts a temporary loopback server, captures every section and sub-section (`systems_network-desktop.png`) at desktop/tablet/mobile/320px widths, and closes the server. It requires an installed browser but no existing service. Inspect those PNGs before publishing or refining further.

Run validation after publishing. It prints document/data SHA-256 values and actionable diagnostics, returns nonzero for errors, and does not execute scripts or judge composition.

```sh
lcars validate --data "$LCARS_DATA"
lcars screenshot --url "$LCARS_URL/s/overview" --width 1440 --height 900 --out desktop.png
lcars screenshot --url "$LCARS_URL/s/overview" --width 390 --height 844 --out mobile.png
lcars screenshot --url "$LCARS_URL/s/overview" --panel overview/energy \
  --expect-panel-hash "$DOCUMENT_HASH" --expect-data-hash "$DATA_HASH" --out energy.png
```

`LCARS_URL` and hashes come from the actual service and validation. Expected hashes require `--panel`; the missing-data sentinel is `missing`. Capture uses installed Chrome/Chromium; pass `--browser` when needed. It waits for the acknowledged displayed revision, latches state, stops live updates, disables optional motion, freezes the display clock, waits for fonts/images/custom stills, and settles layout. The overall deadline is 15 seconds. It writes PNG plus a neighbouring JSON report with viewport, browser, hashes, errors, failed resources, and overflow diagnostics. Failures and mismatches return nonzero, even if old content remains visible.

Inspect the PNGs yourself. A passing capture cannot judge readable hierarchy, meaningful labels, or whether the result answers the user's request. Inspect actual panel widths around 260px, 360px, and 720px as well as page viewports 320px, 390×844, 1024×768, and 1440×900. Check long titles, unbroken text, large values, empty/missing/stale data, wrapped controls, keyboard focus, 200% zoom, and portrait/landscape reflow. Resize an already-mounted interactive panel: state should survive, Canvas should redraw, and focus should remain reachable. Capture renders each section on its own without transitions, so check parking by hand: change a filter or selection, navigate to another section and back (and resize the window while away), and confirm the panel kept its state and drew at the right size. Fix accidental shell or iframe overflow; declared local table/code scrolling is the exception.

Do not deploy, broaden network access, or install the skill as part of ordinary panel authoring. For links shared from a tailnet machine, use its full hostname discovered with `tailscale status --json`, preserve port/path, and verify reachability. Never substitute a guessed hostname or a loopback URL for a link intended for another device.
