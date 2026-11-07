# Atlas status briefing

This is a **demo canvas**. Atlas is a fictional Linux workstation and every reading here is an illustrative fixture, observed at 09:42 UTC on 7 November 2026.

## Needs attention

- **Warning: `/mnt/archive` is 93% full.** At the recent rate of 18.4 GiB a day it fills in about four weeks, but nightly snapshots need roughly 600 GiB of headroom.
- **Warning: `backup-archive.service` failed at 03:12.** The copy stopped with `No space left on device`. Last night's offsite Tailscale push still completed.

## Normal

- CPU is 22% busy with a release build finishing on one thread. Package temperature is 58 °C.
- Memory pressure is negligible: 27.5 GiB in use by applications, 25.8 GiB reclaimable page cache.
- Both NVMe drives report zero media errors and less than 7% rated wear.

## Since the last briefing

Atlas has been up for 23 days since the 6.17.4 kernel update. The only OOM kill this boot was `rust-analyzer` on 3 November, before its memory limit was raised to 8 GiB.
