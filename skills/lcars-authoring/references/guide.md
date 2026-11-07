# Authoring LCARS panels

Authoring contract **1.0**; bridge and JSON schemas use version 1. Discover the app build with `lcars version --json` or the private `/api/version` endpoint. Read this guide with `lcars skill --guide`, print a starter with `lcars skill --starter`, or export the entire portable skill to a new directory with `lcars skill --out ./lcars-authoring`. This is the canonical guide, also served at `/agent-guide`. LCARS provides navigation, frames, shared styles, update times, isolated HTML, live data, and capture tooling. Authors supply ordinary files. An opt-in local scheduler runs adjacent scripts; there is no widget language, built-in upstream connector, or agent runtime.

## Choose a path and format

Find the existing deployment's data root and URL; do not assume this repository is its live content directory. Each immediate folder is a section at `/s/{tab}`. Each direct `.html`, `.md`, or `.txt` file is a panel:

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

Tab and panel IDs are 1–64 lowercase letters, digits, or hyphens. The full panel ID above is `overview/energy`. Duplicate filename stems across formats are invalid. Hidden and temporary files are ignored. Visible unsupported files/directories produce diagnostics. `assets/` never creates panels.

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
{"version":1,"title":"Overview","order":10,"accent":"apricot"}
```

Only these fields are accepted. Sections sort by order then ID; panels sort by order then ID. No site configuration or per-panel sidecar is needed.

## Fit the allocated width

Copy the bundled `assets/starter.html` relative to this skill folder. LCARS injects the base stylesheet and `window.LCARS` before author scripts, and wraps content in `.lcars-content`. It draws the external frame; do not duplicate its elbow and title around your interior.

Start with one column. The iframe viewport is the panel's actual width, so its media queries naturally respond to allocation. Desktop pages can still allocate a narrow column. Use shrinkable CSS Grid tracks (`minmax(0, 1fr)`), allow Flexbox children to shrink, and wrap controls. Convenience classes `.lcars-row` and `.lcars-stack` provide wrapping flex and a single-column grid. The toolkit applies border-box sizing, zero body margin, fluid images/SVG/Canvas, and long-word wrapping.

Avoid fixed page widths, viewport-based heights, absolute positioning for the main layout, clipped focus, and transform scaling. The host measures intrinsic content height, beginning at 320px and clamping it to 80–2400px. Taller panels retain an internal scroller with a diagnostic. Use a sensible compact layout to avoid height/width feedback loops.

SVG diagrams need a `viewBox`, intentional aspect ratio, and accessible description. Draw original geometry. Canvas charts must resize their drawing buffer and redraw when their allocated width changes. Reflow legends, reduce tick density, and offer an accessible data disclosure. Do not shrink a large desktop bitmap until its labels are unreadable.

A useful brief might be: “Show my next seven days, highlight today, make overlaps obvious, and distinguish all-day events in my local timezone.” Choose a calendar grid only when it fits; use an agenda on narrow panels. Keep the same data and selection state across reflow. Controls and details must work with touch and keyboard, without hover. Essential dates, times, and actions stay visible.

For genuinely two-dimensional tables, use a labelled, keyboard-accessible local horizontal scroll region with a visible scroll affordance. Code blocks may also scroll locally. Ordinary prose and controls must reflow; global overflow hiding is not a repair.

## Use the visual contract

The canvas is black. Use warm apricot/orange for primary structure, lilac for supporting structure, and blue/peach for data. The toolkit supplies CSS palette variables `--lcars-background`, `--lcars-text`, `--lcars-muted`, `--lcars-apricot`, `--lcars-orange`, `--lcars-lilac`, `--lcars-blue`, `--lcars-peach`, `--lcars-critical`, and `--lcars-ink`. Prefer these over duplicating hex values.

Use locally bundled Antonio for short labels, headings, and key values through `--lcars-label-font`; use the system sans-serif body stack for prose. Body text begins at 16px with 1.5 line height; labels at 18px; headings around 24–32px; values around 36–56px. Use 8/16/24px spacing, readable muted text, and black text on pastel bands. Keep titles selectable. Avoid gradients, glass, shadows, glow, starfields, fake telemetry, serial-number decoration, or all-uppercase paragraphs.

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

`onState(callback)` delivers visibility, motion preference, capture state, and display clock. Stop optional animation while hidden or motion is off. Initial rendering must work without animation ticks. `deferReady()` called during initial execution returns a completion function for asynchronous setup; call it when setup finishes. Fonts, images, DOM readiness, and initial data are already handled. `onCapture(callback)` lets a diagram draw a still using the frozen display clock; returned promises are awaited. Capture does not record animation or offer timeline controls.

Use local classic scripts and styles in the owned asset directory. No CDN, network fetching, dynamic imports, workers, nested frames, form submission, popup, download, or top navigation. The sandbox intentionally omits same-origin access. Components see only their own data; do not reach into the shell or other frames. Keep useful HTTP(S) source links in Markdown or `lcars:sourceUrl` for host rendering.

Passive SVG resources permit these elements: `svg`, `g`, `path`, `rect`, `circle`, `ellipse`, `line`, `polyline`, `polygon`, `text`, `tspan`, `title`, `desc`, `defs`, `clipPath`, `mask`, `linearGradient`, `radialGradient`, and `stop`.

Allowed attributes are `xmlns`, `viewBox`, `width`, `height`, `x`, `y`, `x1`, `x2`, `y1`, `y2`, `cx`, `cy`, `r`, `rx`, `ry`, `d`, `points`, `fill`, `fill-opacity`, `fill-rule`, `stroke`, `stroke-width`, `stroke-opacity`, `stroke-linecap`, `stroke-linejoin`, `stroke-dasharray`, `opacity`, `transform`, `font-family`, `font-size`, `font-weight`, `text-anchor`, `dominant-baseline`, `id`, `class`, `role`, `aria-label`, `aria-labelledby`, `preserveAspectRatio`, `offset`, `stop-color`, `stop-opacity`, `gradientUnits`, `gradientTransform`, `clip-path`, and `mask`. `url(...)` references must target local `#ids`. Other elements/attributes, scripts, event handlers, animation, external references, directives, and active embedded content are rejected. Symlinks and paths outside the owned asset directory are refused.

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

Every frame shows `Updated …` for the acknowledged displayed document/resources/data revision and an accessible exact-time/source disclosure. Unchanged bytes, polling, pending candidates, and rejected edits do not advance it. File times seed the value at startup; they are publishing evidence, not an audit record.

`Data observed …` uses the producer's observation time. If no observation time was supplied it stays unknown; do not invent one. An elapsed stale threshold adds a visible `Stale` label. Updating a title does not freshen old measurements. A missing live file shows data unavailable. Connection status and last successful check in the shell footer do not describe content freshness.

## Validate, capture, inspect, revise

For a directory-wide check, `lcars snapshot --data "$LCARS_DATA" --out ./captures` validates first, starts a temporary loopback server, captures every section at desktop/tablet/mobile/320px widths, and closes the server. It requires an installed browser but no existing service. Inspect those PNGs before publishing or refining further.

Run validation after publishing. It prints document/data SHA-256 values and actionable diagnostics, returns nonzero for errors, and does not execute scripts or judge composition.

```sh
lcars validate --data "$LCARS_DATA"
lcars screenshot --url "$LCARS_URL/s/overview" --width 1440 --height 900 --out desktop.png
lcars screenshot --url "$LCARS_URL/s/overview" --width 390 --height 844 --out mobile.png
lcars screenshot --url "$LCARS_URL/s/overview" --panel overview/energy \
  --expect-panel-hash "$DOCUMENT_HASH" --expect-data-hash "$DATA_HASH" --out energy.png
```

`LCARS_URL` and hashes come from the actual service and validation. Expected hashes require `--panel`; the missing-data sentinel is `missing`. Capture uses installed Chrome/Chromium; pass `--browser` when needed. It waits for the acknowledged displayed revision, latches state, stops live updates, disables optional motion, freezes the display clock, waits for fonts/images/custom stills, and settles layout. The overall deadline is 15 seconds. It writes PNG plus a neighbouring JSON report with viewport, browser, hashes, errors, failed resources, and overflow diagnostics. Failures and mismatches return nonzero, even if old content remains visible.

Inspect the PNGs yourself. A passing capture cannot judge readable hierarchy, meaningful labels, or whether the result answers the user's request. Inspect actual panel widths around 260px, 360px, and 720px as well as page viewports 320px, 390×844, 1024×768, and 1440×900. Check long titles, unbroken text, large values, empty/missing/stale data, wrapped controls, keyboard focus, 200% zoom, and portrait/landscape reflow. Resize an already-mounted interactive panel: state should survive, Canvas should redraw, and focus should remain reachable. Fix accidental shell or iframe overflow; declared local table/code scrolling is the exception.

Do not deploy, broaden network access, or install the skill as part of ordinary panel authoring. For links shared from a tailnet machine, use its full hostname discovered with `tailscale status --json`, preserve port/path, and verify reachability. Never substitute a guessed hostname or a loopback URL for a link intended for another device.
