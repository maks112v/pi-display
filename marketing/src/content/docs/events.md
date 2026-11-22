---
title: Events
description: Play a separate folder on set dates and hours, once, then go back to normal.
---

An event takes over every screen on a folder for a set time, like a gala, an open house, or a sale weekend. During the event the screens play a subfolder instead of the normal slideshow. On the event's dates, its hours replace the [schedule](/docs/schedules). When the event is over, the screens go back to normal and the event never runs again.

The [event generator](/event) builds the file for you.

## Setting one up

1. Make a subfolder in the media folder and put the event's images and videos in it, like `gala/`.
2. Save the file from the generator next to it, as `gala.event.toml`.

```
Lobby screens/
  display.toml
  gala.event.toml   ← the event
  gala/             ← what it plays
    01-welcome.jpg
    02-sponsors.mp4
  welcome.jpg       ← the normal slideshow
```

The file looks like this:

```toml
folder = "gala"                 # the subfolder to play
dates  = ["2026-12-05"]         # one or more days, YYYY-MM-DD
hours  = ["17:00-23:30"]        # leave out to run all day
```

Any file name ending in `.event.toml` works, and a folder can hold several events.

## When it plays

- **During its hours** the screen is on and plays the event folder, starting on its first file at the start time on every screen. This beats the schedule, so an event can run on a day the screens are normally off.
- **On its dates, outside its hours** the screen is off. The event replaces that day's schedule, including the late hours of a schedule that runs past midnight into it.
- **On every other day** the normal schedule applies.

Hours use the same `"HH:MM-HH:MM"` windows as [schedules](/docs/schedules), in the Pi's local time. A day can have several windows, like `["11:00-14:00", "17:00-22:00"]`. A window that ends earlier than it starts runs past midnight: `"20:00-01:00"` on December 5 plays until 1 AM on December 6, and December 6 otherwise follows the normal schedule.

[`pi-display on` and `off`](/docs/stay-on) and [screen.txt](/docs/screen-txt) still win over an event. They turn the screen on or off, but don't change what an event plays.

If two events overlap, the one that started first keeps playing until it ends.

## Event settings

The event uses the settings from [display.toml](/docs/display-toml). To change them for the event only, add them to the event file:

```toml
folder = "gala"
dates  = ["2026-12-05"]
hours  = ["17:00-23:30"]

image_seconds = 8
shuffle = false
fit = "cover"
mute = false

[durations]
"01-welcome.jpg" = 30
```

`[durations]` starts empty for an event, so a duration in display.toml never applies to a file in the event folder.

## Getting the media onto the Pis

On SMB and Drive, each Pi starts copying the event folder as soon as the event file appears, and keeps it until the event is over. Add the folder and file a day or so early, so every screen has everything before it starts. Make sure the SD card has room for both folders.

If the event folder is empty or missing when the event starts, the screens keep playing the normal slideshow and switch over once the files arrive.

## After the event

The event file does nothing once its last window is over, so it's safe to leave in the folder, but you can delete it and the subfolder. The Pis delete their copy of the folder on their own. To cancel an event early, delete its file.

## Checking it

`pi-display status` lists every event: playing, coming up, or ended. The [logs](/docs/logs) say when an event starts and ends:

```text
event gala.event.toml: playing gala/ until Sat 23:30
```

A file the Pi can't read, like one with a typo in a date, is logged and ignored. On Google Drive, upload the `.toml` file itself, not a Google Doc.
