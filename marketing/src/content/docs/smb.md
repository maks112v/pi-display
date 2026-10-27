---
title: SMB shares
description: Play from a NAS or a shared folder on your network.
---

An SMB share is a folder shared over the network, the kind a NAS, a Windows PC, or a Mac with file sharing turned on can provide. It is the best fit when several screens play the same media and you want to manage it from your own computer.

## Address

Give setup the share and, optionally, a folder inside it:

```text
smb://nas.local/screens
smb://nas.local/screens/lobby
smb://192.168.1.20/media
```

The first part after the host is the share name. Anything after it is a folder inside the share. Add a port if the server doesn't use the standard one: `smb://nas.local:4450/screens`.

The [install page](/install) also accepts Windows-style paths like `\\nas\screens` and adds the `smb://` for you.

## Login

Setup asks for a username and password. Leave the username blank for a guest share.

Use an account that can read the folder. If it can also write, each screen keeps its [log file](/docs/logs) in the share, so you can check every screen without SSH.

> pi-display speaks SMB 2 and 3, which every current NAS and Windows version supports. Very old devices that only speak SMB 1 won't connect.

## Syncing

Every 30 seconds the Pi mirrors the share into `/var/lib/pi-display/media` and plays from that copy:

- New and changed files show up within about 30 seconds.
- Deleted files are removed from the Pi.
- If the share is offline, the screen keeps playing what it already has.

Make sure the SD card has room for the whole folder. A file that would leave less than 256 MB free is skipped and logged, and the rest of the folder keeps syncing.
