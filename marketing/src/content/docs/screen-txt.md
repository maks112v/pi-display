---
title: screen.txt
description: Turn every screen on a folder on or off at once, without touching the Pis.
---

The [screen command generator](/screen) builds this file for you. Drop it into the media folder as `screen.txt`, next to `display.toml`. It holds one command:

```text
off 30m
```

Every Pi playing the folder applies it once, within about 30 seconds for SMB and Drive and within a second for a local folder. It works like running [`pi-display on` or `off`](/docs/stay-on) on each Pi.

## Commands

| Command | What it does |
| --- | --- |
| `off 30m` | Shows black and puts the TV in standby for 30 minutes. |
| `on 2h` | Wakes the TV and keeps the slideshow on for 2 hours, whatever the schedule says. |
| `on 0` | Cancels any on or off command and goes back to the schedule. |
| `on` or `off` | Same as giving it `1h`. |

Durations use Go's format: `45m`, `2h`, `1h30m`. Commands are lowercase. When the time runs out, every screen goes back to its [schedule](/docs/schedules).

## Timing

The duration counts from when the file was saved, not from when each Pi notices it:

- Every screen switches back at the same time, even if one picked up the file later than another.
- A Pi that was offline gets only the time that's left. One that comes back after the time is up ignores the command, so an old file can't turn a screen off later.

## Sending another command

Each saved version of the file runs once. To send a new command, save the file again, even if the words are the same. Deleting the file doesn't cancel anything; save `on 0` instead.

Between `screen.txt` and someone running `pi-display on` or `off` on a Pi, whichever is newer wins.

## Checking it worked

Each Pi writes what it did to its [logs](/docs/logs):

```text
screen.txt: Screen off for 30m, until Tue 19:30, then back on schedule
```

On an [SMB share](/docs/smb) that line also lands in each screen's log file on the share, so you can check every screen without SSH. A line the Pi can't read, like `of 30m`, is logged and ignored.

## Google Drive

Upload a plain `.txt` file, made on your computer or downloaded from the [generator](/screen). A Google Doc named `screen.txt` won't work, because Drive doesn't store it as a text file.
