---
title: Staying on for an event
description: Keep the slideshow running outside its schedule.
---

Outside its [schedule](/docs/schedules) the screen shows black. To keep the slideshow running anyway, SSH into the Pi and run:

```sh
pi-display on        # on for 1 hour
pi-display on 3h30m  # on for any duration
pi-display on 0      # cancel and go back to the schedule
```

Durations use Go's format: `90m`, `2h`, `1h30m`.

It takes effect within 5 seconds and survives a reboot. When it runs out, the screen goes back to its schedule at the next slide.
