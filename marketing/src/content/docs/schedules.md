---
title: Schedules
description: Turn the screen off outside set hours.
---

Add a `[schedule]` to [display.toml](/docs/display-toml), or pick hours in the **Schedule** step of the [config generator](/config). Without a schedule, the screen is always on.

## Same hours every day

```toml
[schedule]
on = "07:00"
off = "22:00"
```

Times are 24-hour `HH:MM` in the Pi's local time.

## By day of week

List the hours for each day as `"HH:MM-HH:MM"` windows. Days you leave out stay off. A day can have more than one window:

```toml
[schedule]
sun = ["09:00-18:00"]
fri = ["07:00-12:00", "17:00-22:00"]
```

Days are `sun`, `mon`, `tue`, `wed`, `thu`, `fri`, and `sat`. A schedule uses either `on` and `off` or days, not both, and needs at least one window.

## Overnight hours

An off time earlier than the on time runs past midnight. This keeps a screen on from 6 PM to 2 AM:

```toml
[schedule]
on = "18:00"
off = "02:00"
```

In a day's list, the window runs into the next morning: `fri = ["20:00-01:00"]` is on until 1 AM Saturday. The on and off times must differ.

## Outside the schedule

The screen shows black, and the Pi puts the TV in standby over HDMI-CEC. When the schedule comes back on, it wakes the TV and switches it to the Pi's input. Changes to the schedule take effect at the next slide.

CEC has to be turned on in the TV's settings. Brands give it their own name: Anynet+ (Samsung), Bravia Sync (Sony), SimpLink (LG), or CEC. If the TV doesn't respond, the [logs](/docs/logs) say why. To leave the TV alone and only show black, set `tv_power = false` in [display.toml](/docs/display-toml). Either way, when the Pi powers on during the schedule it sends one wake over HDMI-CEC, in case the TV came back from a power cut in standby. To keep the slideshow running for an evening event, or turn the screen off early, see [Turning the screen on or off](/docs/stay-on).

> Set the Pi's timezone with `sudo raspi-config` (under Localisation Options) so the schedule matches the wall clock.
