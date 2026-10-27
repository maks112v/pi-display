---
title: Schedules
description: Turn the screen off outside set hours.
---

Add a `[schedule]` to [display.toml](/docs/display-toml), or turn on **Schedule** in the [guided setup](/config):

```toml
[schedule]
on = "07:00"
off = "22:00"
```

Times are 24-hour `HH:MM` in the Pi's local time. Without a schedule, the screen is always on.

## Overnight hours

An off time earlier than the on time runs past midnight. This keeps a screen on from 6 PM to 2 AM:

```toml
[schedule]
on = "18:00"
off = "02:00"
```

The on and off times must differ.

## Outside the schedule

The screen shows black. Changes to the schedule take effect at the next slide. To keep the slideshow running for an evening event, see [Staying on for an event](/docs/stay-on).

> Set the Pi's timezone with `sudo raspi-config` (under Localisation Options) so the schedule matches the wall clock.
