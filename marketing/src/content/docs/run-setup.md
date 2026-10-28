---
title: Run setup
description: Point Pi Display at your media folder and start the slideshow on every boot.
---

From the folder you downloaded Pi Display into, run:

```sh
sudo ./pi-display setup
```

Run it with `sudo` from your normal user, not as root. The slideshow runs as that user.

## What setup does

1. Asks for the media folder: an [SMB share](/docs/smb), a [Google Drive folder](/docs/google-drive), or a [local folder](/docs/local-folder).
2. Connects and reports how many images and videos it found. A wrong link or password stops setup here.
3. Installs mpv, the video player, and v4l-utils, which turns the TV off and on with the [schedule](/docs/schedules), if they are missing.
4. Installs itself to `/usr/local/bin/pi-display` and adds a service that starts the slideshow on every boot. You can delete the downloaded `./pi-display` afterwards.
5. If the Pi boots to a desktop, offers to boot to the console instead. Say yes: the slideshow needs the screen to itself. You can still open the desktop later with `sudo pi-display desktop`. See [Fixing a Pi on site](/docs/on-site#the-desktop).
6. Sets saved Wi-Fi networks to keep reconnecting forever, so a Pi that boots before the router after a power cut still gets back online. Networks added later need the same. See [Fixing a Pi on site](/docs/on-site).
7. If the Pi has the boot splash (the desktop image does), sets the boot logo to the Pi Display icon. On Raspberry Pi OS Lite this step is skipped.

The slideshow starts as soon as setup finishes.

## Skip the questions

The [install page](/install) builds a setup code from your folder and login. Run the command it gives you instead:

```sh
sudo ./pi-display setup <code>
```

With a code, setup turns the desktop off at boot without asking. Running it again on a Pi keeps that Pi's playback settings; set those in the [config generator](/config).

> The code includes the password, encoded but not encrypted. Treat it like the password.

## Change the folder later

Run setup again any time to point the screen at a different folder:

```sh
sudo pi-display setup
```
