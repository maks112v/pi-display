---
title: Updating
description: Install the latest release on a running screen.
---

```sh
sudo pi-display update
```

Update downloads the build for your Pi from the latest GitHub release, checks it against the release's `SHA256SUMS`, swaps it in, and restarts the slideshow. Your folder, login, and settings stay as they are.

Check which release is installed with:

```sh
pi-display version
```

Release notes for every version are on the [releases page](https://github.com/maks112v/pi-display/releases).
