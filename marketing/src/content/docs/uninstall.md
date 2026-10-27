---
title: Uninstall
description: Remove pi-display and everything it added to the Pi.
---

Over SSH on the Pi, stop the slideshow and remove the service:

```sh
sudo systemctl disable --now pi-display
sudo rm /etc/systemd/system/pi-display.service
sudo systemctl daemon-reload
```

Then remove the program, its settings, and the mirrored media:

```sh
sudo rm /usr/local/bin/pi-display
sudo rm -rf /etc/pi-display /var/lib/pi-display
```

> `/etc/pi-display` holds the SMB password. Removing it is the step that takes the login off the Pi.

## Optional cleanup

If setup switched the Pi from a desktop to the console, switch it back:

```sh
sudo systemctl set-default graphical.target
sudo reboot
```

Setup installed mpv if it was missing. Remove it if nothing else uses it:

```sh
sudo apt-get remove mpv
```

## On the share

With an SMB share, each screen left a `<hostname>.pi-display.log` in the folder. Delete it from any computer. Your media files are never changed.

To wipe the Pi completely instead, reflash the SD card as in [Prepare the Pi](/docs/prepare-the-pi).
