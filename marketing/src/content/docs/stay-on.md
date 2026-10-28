---
title: Turning the screen on or off
description: Override the schedule for an event, or turn the screen off for a while.
---

To override the [schedule](/docs/schedules) for a while, SSH into the Pi and run:

```sh
pi-display on        # on for 1 hour
pi-display on 3h30m  # on for any duration
pi-display off 2h    # off for any duration
pi-display on 0      # cancel either and go back to the schedule
```

Durations use Go's format: `90m`, `2h`, `1h30m`.

`off` shows black and puts the TV in standby over HDMI-CEC. It works with or without a schedule. `on` wakes the TV and switches it to the Pi's input, even if someone turned it off with the remote. The TV part needs `tv_power`, which is on by default. See [Schedules](/docs/schedules#outside-the-schedule).

A new command replaces the last one, takes effect within a second, and survives a reboot. When it runs out, the screen goes back to its schedule.

## From the shared folder

To switch every screen that plays a folder at once, put a file named `screen.txt` at the top of the folder, next to `display.toml`, with one command in it:

```text
off 30m
```

`on 2h`, `off 1h30m`, and `on 0` (cancel and go back to the schedule) work the same way as on the command line. Each Pi picks it up within about 30 seconds and applies it once. To send another command, even the same one, save the file again.

The duration counts from when the file was saved, so every screen switches back at the same time. A Pi that was offline gets only the time that's left, and one that comes back after the time is up ignores the command. Whichever is newer wins between `screen.txt` and someone running `pi-display on` or `off` on a Pi.

Each Pi logs what it did, like `screen.txt: Screen off for 30m, until Tue 19:30, then back on schedule`, and a line it can't read is logged and ignored. On Google Drive, upload a plain `.txt` file; a Google Doc won't work.
