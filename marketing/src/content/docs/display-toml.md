---
title: display.toml
description: Change timing, fit, and hours for every screen that plays from a folder, without touching the Pis.
---

The [config generator](/config) builds this file for you. Drop it into the media folder as `display.toml`. Each Pi re-reads it before every slide, so changes show up within one slide, plus up to 30 seconds of sync for SMB and Drive.

```toml
image_seconds = 10
mute = true
fit = "contain"
shuffle = false

[durations]
"intro.jpg" = 30

[schedule]
on = "07:00"
off = "22:00"
```

Every key is optional. Leave out anything you don't want to change.

## Settings

<dl>
  <dt><code>image_seconds</code> <span>number, default 10</span></dt>
  <dd>How long each image stays on screen, from 1 to 86400. Videos always play to the end.</dd>
  <dt><code>fit</code> <span>"contain" or "cover", default "contain"</span></dt>
  <dd>Contain letterboxes to show the whole image. Cover fills the screen and crops the edges.</dd>
  <dt><code>mute</code> <span>boolean, default true</span></dt>
  <dd>Mutes video audio.</dd>
  <dt><code>shuffle</code> <span>boolean, default false</span></dt>
  <dd>Plays every file once per loop in a new random order. The last file of one loop never plays first in the next.</dd>
  <dt><code>[durations]</code> <span>file name to seconds</span></dt>
  <dd>Gives specific images more or less time than <code>image_seconds</code>. Names must match the file exactly.</dd>
  <dt><code>[schedule]</code> <span>on and off, or windows per day</span></dt>
  <dd>The hours the screen is on: <code>on</code> and <code>off</code> for every day, or lists like <code>sun = ["09:00-18:00"]</code> per weekday. See <a href="/docs/schedules">Schedules</a>.</dd>
</dl>

## If something is wrong

The file is read strictly. An unknown key, a wrong type, or an out-of-range value means the whole file is ignored, the last good settings stay in effect, and the error is [logged](/docs/logs). Nothing in it is passed to mpv as raw options.

## One Pi only

To give a single screen its own settings, choose **Apply to this Pi only** in the config generator and run the command it shows:

```sh
sudo pi-display config <code>
```

It saves the settings on that Pi and restarts the slideshow. The folder and login stay as they are. A `display.toml` in the folder is still applied on top: it only overrides the keys it sets, and its `[durations]` are added to the Pi's.
