---
title: Logs
description: See what a screen is doing, on the Pi or from the share.
---

## On the Pi

Pi Display logs to the system journal. Over SSH, run:

```sh
pi-display logs
```

It's a shortcut for `journalctl -u pi-display -f`. To page through earlier logs, run `journalctl -u pi-display` without `-f`. Sync errors, skipped files, and problems in [display.toml](/docs/display-toml) all show up here.

## In the SMB share

With an [SMB share](/docs/smb), each screen also writes its log to the share after every sync, as `<hostname>.pi-display.log`. You can check every screen from your own computer without SSH.

- Lines logged while the share is unreachable are held, up to 500, and written once it is back.
- The file starts over after 1 MB.
- `.log` files never play.

The login needs write access to the folder. With a read-only login, logs stay in the journal only.
