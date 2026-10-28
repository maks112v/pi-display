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

Turn on `shuffle` in [display.toml](/docs/display-toml) to play everything once per loop in a new random order instead.

## Timing

- Images show for `image_seconds` (10 by default). [Longer slides](/docs/display-toml) can be set per file.
- Videos play once through, then the slideshow moves on.

## What's ignored

- Files inside subfolders. Only the top level of the folder plays.
- Hidden files, whose names start with a dot.

The folder is re-read before every file, so changes show up at the next slide, plus up to 30 seconds of sync for SMB and Drive.

## Playback

[mpv](https://mpv.io) decodes everything and draws straight to the screen with hardware decoding. Pi Display decides what plays and for how long.
