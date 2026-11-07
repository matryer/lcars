# Services

Demo fixture. **287** systemd units loaded, **1** failed, **14** timers scheduled.

## Failed

- **Warning: `backup-archive.service`** failed at 03:12 UTC after 12 minutes. Exit status 1: `rsync: write failed on "/mnt/archive/snapshots/2026-11-07/home.zst": No space left on device (28)`.

## Next timers

- `backup-archive.timer` at 03:00 tomorrow, will fail again unless space is freed
- `fstrim.timer` on Monday at 00:00
- `btrfs-scrub@home.timer` on 1 December, last scrub found 0 errors
- `docker-prune.timer` at 18:00 today, last reclaimed 41.2 GiB

## Recent journal warnings

- `09:18` `kernel: nvme1n1: I/O 412 QID 7 timeout, completion polled` (once, no retries needed)
- `03:12` `backup-archive.service: Failed with result 'exit-code'`
- `Nov 03 16:40` `systemd-oomd: Killed rust-analyzer (4.9 GiB) due to memory pressure`

Free space with `btrfs filesystem du` and `ncdu /mnt/archive` before tonight's run.
