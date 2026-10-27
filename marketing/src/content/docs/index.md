---
title: Introduction
description: Turn a Raspberry Pi into a full-screen slideshow of every image and video in a folder.
---

pi-display plays every image and video in a folder, full screen, in filename order. The folder can be an SMB share on your network, a public Google Drive folder, or a folder on the Pi itself. The Pi boots straight into the slideshow whenever it gets power.

## How it fits together

- **You keep the media where you already work.** Drop files into the shared folder from any computer. Each screen picks up changes within about 30 seconds.
- **Each Pi keeps its own copy.** Files are mirrored onto the SD card, so the slideshow starts at boot before the network is up and keeps playing if the share goes offline.
- **One file controls playback.** An optional [display.toml](/docs/display-toml) in the folder sets timing, fit, shuffle, and screen hours for every screen that plays from it.

## Set up a screen

The quickest way is the [install page](/install): it walks through installing and picking a folder, then gives you one command to run on the Pi. The [config generator](/config) then sets timing and schedules.

To do it by hand, follow these pages in order:

1. [Prepare the Pi](/docs/prepare-the-pi): flash the SD card.
2. [Install](/docs/install): download pi-display.
3. [Run setup](/docs/run-setup): point it at your folder.
