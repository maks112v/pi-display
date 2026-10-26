# pi-display

Turns a Raspberry Pi into a full-screen slideshow of every image and video in a folder, played in filename order. The folder can be local or on an SMB share. The Pi boots straight into the slideshow whenever it gets power.

## Install

Use Raspberry Pi OS Lite (Trixie or newer). Build on any machine:

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o dist/pi-display .
scp dist/pi-display pi@raspberrypi.local:
```

Then on the Pi:

```sh
sudo ./pi-display setup
```

Setup asks for the folder (`smb://nas.local/share/path` or `/home/pi/media`) and, for SMB, a username and password. It tests the connection, installs mpv if needed, then installs and starts a systemd service. Re-run it any time to point at a different folder.

Logs: `journalctl -u pi-display -f`

## How it works

- SMB folders are mirrored into `/var/lib/pi-display/media` every 30 seconds. Playback runs from that copy, so the slideshow starts at boot before the network is up and keeps going if the share goes offline. Make sure the SD card has room for the whole folder.
- Only files at the top level of the folder are used. Subfolders and hidden files are ignored.
- Order is natural filename order: `2.jpg` plays before `10.jpg`.
- The folder and `display.toml` are re-read before every file, so changes show up within one slide (plus up to 30 seconds of sync).
- mpv does the decoding and draws straight to the screen with hardware decoding. `pi-display` decides what plays and for how long.

Images: jpg, jpeg, png, webp, bmp, gif. Videos: mp4, m4v, mov, mkv, webm, avi. Videos play once through, then the slideshow moves on.

## display.toml

Drop an optional `display.toml` into the folder:

```toml
image_seconds = 10   # how long each image shows (default 10)
mute = true          # mute video audio (default true)
fit = "contain"      # "contain" letterboxes, "cover" fills and crops (default "contain")

# Per-image overrides, in seconds
[durations]
"intro.jpg" = 30
```

The file is parsed strictly. If it has an unknown key, a wrong type, or a value out of range, the whole file is ignored, the last good settings stay in effect, and the error is logged. Nothing in it is passed to mpv as raw options.

## Files on the Pi

| Path | What |
| --- | --- |
| `/etc/pi-display/config.json` | Source and SMB credentials (0600) |
| `/usr/local/bin/pi-display` | Installed binary |
| `/etc/systemd/system/pi-display.service` | Boot service |
| `/var/lib/pi-display/media` | Mirrored SMB files |
