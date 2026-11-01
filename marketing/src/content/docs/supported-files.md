---
title: Supported files
description: Which files play, and in what order.
---

| Type | Extensions |
| --- | --- |
| Images | jpg, jpeg, png, webp, bmp, gif |
| Videos | mp4, m4v, mov, mkv, webm, avi |

Extensions are matched case-insensitively. Anything else in the folder, including `display.toml` and `.log` files, is ignored.

Animated WebP files, like the ones GIPHY serves, don't play: the video decoder on Raspberry Pi OS only reads still WebP. Pi Display skips them and says so in the [logs](/docs/logs) and `pi-display status`. Save the animation as MP4 or GIF instead.

## Order

Files play in natural filename order, so `2.jpg` plays before `10.jpg`. Number your files to control the order:

```text
01-welcome.jpg
02-menu.png
03-promo.mp4
```

Turn on `shuffle` in [display.toml](/docs/display-toml) to play everything once per loop in a new random order instead. Every screen on the folder gets the same order.

Screens on the same folder play in step. Playback is timed by the Pi's clock, which Raspberry Pi OS syncs over the network, so a Pi that reboots joins at the slide the others are on, partway into a video if need be.

## Timing

- Images show for `image_seconds` (10 by default). [Longer slides](/docs/display-toml) can be set per file.
- Videos play once through, then the slideshow moves on.

## What's ignored

- Files inside subfolders. Only the top level of the folder plays.
- Hidden files, whose names start with a dot.

The folder is checked every second. A change cuts off the current slide and jumps straight to where the new lineup puts every screen. On SMB and Drive, new files wait about a minute first so every screen has them.

## Playback

[mpv](https://mpv.io) decodes everything and draws straight to the screen with hardware decoding. Pi Display decides what plays and for how long.
