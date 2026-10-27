---
title: Prepare the Pi
description: Flash an SD card with Raspberry Pi OS Lite and get SSH access.
---

Flash an SD card with [Raspberry Pi Imager](https://www.raspberrypi.com/software/).

## Choose the OS

Pick **Raspberry Pi OS Lite (64-bit)**, Trixie or newer. Lite has no desktop, which is what you want: the slideshow draws straight to the screen.

> Use the 32-bit Lite image only on a Pi older than a Pi 3 or Zero 2 W.

## Customise

Before writing the card, open the customisation settings and:

- set a username and password
- add your Wi-Fi network if the Pi is not on Ethernet
- set your timezone, so [schedules](/docs/schedules) match the wall clock
- turn on SSH

## Boot and connect

Plug the Pi into the display and power it on. Once it has booted, SSH in as the user you created:

```sh
ssh pi@raspberrypi.local
```

Replace `pi` and `raspberrypi` with the username and hostname you chose. Next, [install pi-display](/docs/install).
