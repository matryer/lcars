---
name: lcars-authoring
description: Create or update file-based panels and adjacent scheduled data scripts in an existing LCARS canvas, then validate and inspect captures. Use for LCARS content authoring, not deployment or upstream account changes.
---

# LCARS authoring

Authoring contract: **1.0** (bridge and file schemas: version 1). Run `lcars version --json` for the installed app build and supported contract.

Read [the authoring guide](references/guide.md) for file layout, metadata, the data helper, adjacent scheduled scripts, publishing, and capture checks. Start HTML panels from [starter.html](assets/starter.html). The running app serves the same guide at `/agent-guide`. Without repository access, use `lcars skill --guide` and `lcars skill --starter`. Export this entire folder with `lcars skill --out ./lcars-authoring` when an explicit installation is wanted.

Obtain the existing data root and service URL from the environment or user. Own one `tab/panel` and its `assets/panel/` directory; coordinate before replacing another author's files. Preserve the requested presentation and existing source-access permissions. Displaying information does not authorize changing its upstream source.

Publish files atomically, run `lcars validate`, capture the accepted panel at desktop and mobile widths, inspect both PNGs, and revise as needed. Check the actual iframe width as well as the page width: one desktop column may be phone-sized. A successful validation or capture is evidence of mechanics, not visual quality.

For recurring data, put `<panel>.<interval>.py` (or `.js`, `.sh`, `.ps1`) beside the HTML panel. Follow the guide’s scheduled-script contract: one JSON envelope on stdout, diagnostics on stderr, and LCARS owns the output file. Execution requires an already authorized `--run-scripts` service; validation never executes scripts. Keep using `LCARS.onData()` in the panel; the shell handles SSE and recovery.

Show times with `<lcars-time datetime="…" label="…">`, not hand-formatted text. Put secondary detail behind `LCARS.openModal({title, body, fields})`, opened from a click.

This skill neither deploys LCARS nor installs itself. Copy the entire folder explicitly into a supported skill directory when desired; agents without skills can read the guide directly.
