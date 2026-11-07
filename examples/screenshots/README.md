# README screenshots

These are desktop viewport captures taken with the current binary. The product banner is rendered from a dedicated LCARS canvas; the other images show the bundled fictional examples. To refresh them, build with `mise run build`, then run these temporary local servers in separate terminals:

```sh
./lcars serve --data ./examples/content --listen 127.0.0.1:18087
./lcars serve --data ./examples/machine/content --listen 127.0.0.1:18088
```

Capture with Chrome or Chromium installed:

```sh
./lcars screenshot --url http://127.0.0.1:18087/s/overview --width 1440 --height 1000 --out examples/screenshots/overview.png
./lcars screenshot --url http://127.0.0.1:18087/s/week --width 1440 --height 1000 --out examples/screenshots/week.png
./lcars screenshot --url http://127.0.0.1:18088/s/compute --width 1440 --height 1000 --out examples/screenshots/machine.png
```

Inspect each image and its neighbouring JSON capture report before committing the PNGs. The reports are local verification output and need not be committed. Stop both servers when finished. These loopback addresses are only for local capture; no service is published.

## Product banner

The source canvas lives in `examples/readme/content`. Serve it temporarily:

```sh
./lcars serve --data ./examples/readme/content --listen 127.0.0.1:18089
```

Then capture the README header:

```sh
./lcars screenshot --url http://127.0.0.1:18089/s/lcars --width 1440 --height 320 --out examples/screenshots/banner.png
```

The compact viewport captures the product message above the panel's update footer. The image uses the actual LCARS shell, frame, font, and toolkit palette. Stop the temporary server after capture.
