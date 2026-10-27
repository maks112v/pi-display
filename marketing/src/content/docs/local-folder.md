---
title: Local folder
description: Play files stored on the Pi itself.
---

A folder on the Pi works when a screen stands alone, or when something else already puts files onto the Pi, such as a USB drive or your own sync tool.

## Set it up

Create the folder, then give setup its absolute path:

```sh
mkdir -p ~/media
sudo ./pi-display setup
```

```text
Media folder (smb://host/share/path, a public Google Drive folder link, or /local/path): /home/pi/media
```

Local folders aren't offered on the [install page](/install). Use the questions in `pi-display setup` instead.

## Add files

Copy files in from another computer with `scp`:

```sh
scp *.jpg pi@raspberrypi.local:~/media/
```

Local folders are played in place, not copied, so changes show up at the next slide. The slideshow runs as the user who ran setup, so that user needs to be able to read the folder.
