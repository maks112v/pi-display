---
title: Commands
description: Everything Pi Display can do from the command line.
---

Run these over SSH on the Pi. With no network, plug a keyboard into the Pi and press Ctrl+Alt+T to get a console. See [Fixing a Pi on site](/docs/on-site).

| Command | What it does |
| --- | --- |
| `sudo pi-display setup` | Asks for a media folder, then installs and starts the slideshow. See [Run setup](/docs/run-setup). |
| `sudo pi-display setup <code>` | Same, using a code from the [install page](/install). |
| `sudo pi-display config <code>` | Applies playback settings from the [config generator](/config) to this Pi only. See [display.toml](/docs/display-toml#one-pi-only). |
| `sudo pi-display update` | Installs the latest release, keeping your settings. See [Updating](/docs/updating). |
| `sudo pi-display uninstall` | Removes the slideshow and everything setup added. See [Uninstall](/docs/uninstall). |
| `pi-display on [duration]` | Turns the screen on for a while, whatever its schedule says. See [Turning the screen on or off](/docs/stay-on). |
| `pi-display off [duration]` | Turns the screen off and puts the TV in standby for a while. See [Turning the screen on or off](/docs/stay-on). |
| `sudo pi-display start` | Restarts the slideshow after the console or desktop. See [Fixing a Pi on site](/docs/on-site). |
| `sudo pi-display desktop` | Stops the slideshow and starts the desktop until the next reboot. See [Fixing a Pi on site](/docs/on-site#the-desktop). |
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
