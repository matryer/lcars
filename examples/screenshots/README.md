# README screenshots

These are desktop viewport captures of the bundled fictional examples, taken with the current binary. To refresh them, build with `mise run build`, then run these temporary local servers in separate terminals:

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
