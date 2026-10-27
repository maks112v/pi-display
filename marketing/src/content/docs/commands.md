---
title: Commands
description: Everything pi-display can do from the command line.
---

Run these over SSH on the Pi.

| Command | What it does |
| --- | --- |
| `sudo pi-display setup` | Asks for a media folder, then installs and starts the slideshow. See [Run setup](/docs/run-setup). |
| `sudo pi-display setup <code>` | Same, using a code from the [install page](/install). |
| `sudo pi-display config <code>` | Applies playback settings from the [config generator](/config) to this Pi only. See [display.toml](/docs/display-toml#one-pi-only). |
| `sudo pi-display update` | Installs the latest release, keeping your settings. See [Updating](/docs/updating). |
| `sudo pi-display uninstall` | Removes the slideshow and everything setup added. See [Uninstall](/docs/uninstall). |
| `pi-display on [duration]` | Keeps the screen on outside its schedule. See [Staying on for an event](/docs/stay-on). |
| `pi-display status` | Shows the version, media source, whether the service is running, and whether the screen should be on. Start here when a screen misbehaves. |
| `pi-display logs` | Follows the slideshow's logs. Same as `journalctl -u pi-display -f`. |
| `pi-display version` | Shows the installed release. |
| `pi-display run` | Runs the slideshow in the foreground. The boot service uses this; you normally don't need it. |

## The service

Setup installs a systemd service named `pi-display`. The usual systemd commands work:

```sh
sudo systemctl restart pi-display  # restart the slideshow
sudo systemctl stop pi-display     # stop until the next boot
journalctl -u pi-display -f        # watch the logs
```
