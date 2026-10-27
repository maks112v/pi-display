---
title: Uninstall
description: Remove Pi Display and everything it added to the Pi.
---

Over SSH on the Pi, run:

```sh
sudo pi-display uninstall
```

This stops the slideshow and removes the service, the program, its settings, and the mirrored media. If setup set the boot logo, it puts the Raspberry Pi one back. If the Pi has a desktop that is switched off, it asks whether to turn it back on. If mpv is installed, it asks whether to remove it too.

> The `uninstall` command arrived in v0.0.5. On an older release, run `sudo pi-display update` first, or use the manual steps below.

## Manual steps

To do the same by hand, stop the slideshow and remove the service:

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

### Boot logo

If setup set the boot logo, put the Raspberry Pi one back and remove the theme. This rebuilds the initramfs, which takes a minute:

```sh
sudo plymouth-set-default-theme -R pix
sudo rm -rf /usr/share/plymouth/themes/pi-display
```

### Optional cleanup

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
