---
title: Files on the Pi
description: Where pi-display keeps its program, settings, and media.
---

| Path | What |
| --- | --- |
| `/usr/local/bin/pi-display` | The installed program |
| `/etc/pi-display/config.json` | Media folder, SMB login, and settings from setup. Readable only by the user the slideshow runs as. |
| `/etc/systemd/system/pi-display.service` | The service that starts the slideshow at boot |
| `/var/lib/pi-display/media` | The mirrored copy of an SMB or Drive folder |
| `/var/lib/pi-display/awake-until` | When a `pi-display on` override ends |

Setup also installs `mpv` with apt, and may switch the Pi to boot to the console instead of a desktop. [Uninstall](/docs/uninstall) covers undoing both.
