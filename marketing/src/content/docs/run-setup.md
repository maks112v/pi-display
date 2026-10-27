---
title: Run setup
description: Point pi-display at your media folder and start the slideshow on every boot.
---

From the folder you downloaded pi-display into, run:

```sh
sudo ./pi-display setup
```

Run it with `sudo` from your normal user, not as root. The slideshow runs as that user.

## What setup does

1. Asks for the media folder: an [SMB share](/docs/smb), a [Google Drive folder](/docs/google-drive), or a [local folder](/docs/local-folder).
2. Connects and reports how many images and videos it found. A wrong link or password stops setup here.
3. Installs mpv, the video player, if it is missing.
4. Installs itself to `/usr/local/bin/pi-display` and adds a service that starts the slideshow on every boot. You can delete the downloaded `./pi-display` afterwards.
5. If the Pi boots to a desktop, offers to boot to the console instead. Say yes: the slideshow needs the screen to itself.

The slideshow starts as soon as setup finishes.

## Skip the questions

The [guided setup](/config) builds a setup code from your folder, login, and playback settings. Run the command it gives you instead:

```sh
sudo ./pi-display setup <code>
```

With a code, setup turns the desktop off at boot without asking.

> The code includes the password, encoded but not encrypted. Treat it like the password.

## Change the folder later

Run setup again any time to point the screen at a different folder:

```sh
sudo pi-display setup
```
