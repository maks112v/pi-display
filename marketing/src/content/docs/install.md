---
title: Install
description: Download the latest Pi Display release onto the Pi.
---

On the Pi, run:

```sh
curl -fsSLo pi-display "https://github.com/maks112v/pi-display/releases/latest/download/pi-display-$(dpkg --print-architecture)"
chmod +x pi-display
```

## What these do

- `curl -fsSLo pi-display` downloads the latest release from GitHub and saves it as a file named `pi-display`.
- `$(dpkg --print-architecture)` asks the Pi whether it runs 64-bit (`arm64`) or 32-bit (`armhf`) Raspberry Pi OS, so you get the matching build.
- `chmod +x pi-display` marks the file as a program so the Pi is allowed to run it.

This only downloads the file into your current folder. Nothing is installed until you [run setup](/docs/run-setup), which copies it to `/usr/local/bin`.

Every release is listed on the [releases page](https://github.com/maks112v/pi-display/releases). Each one includes a `SHA256SUMS` file if you want to check the download.

## Build from source

With Go installed on any computer:

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o pi-display .
```

Use `GOARCH=arm GOARM=6` for 32-bit Raspberry Pi OS, then copy the file to the Pi.
