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
- add your Wi-Fi network if the Pi is not on Ethernet. If it changes later, see [Fixing a Pi on site](/docs/on-site).
- set your timezone, so [schedules](/docs/schedules) match the wall clock
- turn on SSH

## Boot and connect

> **Recommended:** use [Raspberry Pi Connect](https://www.raspberrypi.com/software/connect/). It gives you a shell on the Pi from your browser, so you can run the setup commands without finding its IP address or setting up SSH keys, and you can reach it remotely once it is mounted on the wall. Turn it on in Imager's customisation settings, or install it later with `sudo apt install rpi-connect-lite` and `rpi-connect signin`.

Plug the Pi into the display and power it on. Once it has booted, SSH in as the user you created:

```sh
ssh pi@raspberrypi.local
```

Replace `pi` and `raspberrypi` with the username and hostname you chose. Next, [install Pi Display](/docs/install).
