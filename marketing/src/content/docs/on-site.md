---
title: Fixing a Pi on site
description: Get to the console or desktop with a keyboard, for example to change Wi-Fi.
---

When a Pi moves to a new building or the Wi-Fi changes, it can't be reached over SSH. Fix it from the Pi itself:

1. Plug a USB keyboard into the Pi and press **Ctrl+Alt+T**. Ctrl+Alt+F1 through F12 work too.
2. The slideshow stops and the screen shows the console login. Press Enter if you don't see a prompt, then log in with the Pi's account.
3. Change Wi-Fi with `sudo raspi-config`, under System Options, Wireless LAN. Then make the new network reconnect forever after a power cut, like setup does: `sudo nmcli connection modify "<network name>" connection.autoconnect-retries 0`
4. Bring the slideshow back:

```sh
sudo pi-display start
```

A reboot brings it back too.

## The desktop

If the Pi has the desktop installed (not Raspberry Pi OS Lite), you can use it instead of the console:

```sh
sudo pi-display desktop
```

The slideshow stops and the desktop starts, so you can change Wi-Fi and other settings with a mouse. It lasts until the next reboot, and the Pi still boots into the slideshow. To go back sooner, open a terminal and run `sudo pi-display start`.

## While it's offline

Media already on the Pi keeps playing from its local copy, so the screen shows the last synced files until the network is back. New files arrive within 30 seconds once it is.
