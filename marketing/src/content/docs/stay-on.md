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

A new command replaces the last one, takes effect at the next slide, and survives a reboot. When it runs out, the screen goes back to its schedule.
