# pi-display

Turns a Raspberry Pi into a full-screen slideshow of every image and video in a folder, played in filename order. The folder can be local, on an SMB share, or a public Google Drive folder. The Pi boots straight into the slideshow whenever it gets power.

## Getting started

### 1. Prepare the Pi

Flash an SD card with [Raspberry Pi Imager](https://www.raspberrypi.com/software/):

- **OS:** Raspberry Pi OS Lite (64-bit), Trixie or newer. Use the 32-bit Lite image only on a Pi older than a Pi 3 or Zero 2 W.
- **Customisation:** set a username and password, Wi-Fi if you are not on Ethernet, your timezone, and turn on SSH.

Plug the Pi into the display, boot it, and SSH in as the user you created.

### 2. Download pi-display

```sh
curl -fsSLo pi-display "https://github.com/maks112v/pi-display/releases/latest/download/pi-display-$(dpkg --print-architecture)"
chmod +x pi-display
```

This picks the right build for your OS (`arm64` or `armhf`). Every release is listed on the [releases page](https://github.com/maks112v/pi-display/releases).

### 3. Run setup

```sh
sudo ./pi-display setup
```

Run it with `sudo` from your normal user, not as root. The slideshow runs as that user. Setup:

1. Installs mpv if it is missing.
2. Asks for the media folder. Any of these work:
   - SMB share: `smb://nas.local/share/path`. You are then asked for a username (blank for guest) and password.
   - Public Google Drive folder: `https://drive.google.com/drive/folders/...`. No sign-in, but the folder must be shared as "Anyone with the link".
   - Local folder: an absolute path like `/home/pi/media`.
3. Connects and reports how many images and videos it found. A wrong link or password stops setup here.
4. Installs itself to `/usr/local/bin/pi-display` and adds a service that starts the slideshow on every boot. You can delete the downloaded `./pi-display` afterwards.
5. If the Pi boots to a desktop, offers to boot to console instead. Say yes: the slideshow needs the screen to itself.

The slideshow starts as soon as setup finishes, and again whenever the Pi gets power. Re-run `sudo pi-display setup` any time to point it at a different folder.

### 4. Add media

Put images and videos in the folder. New files show up within about 30 seconds. To change timing, add a [`display.toml`](#displaytoml) to the same folder.

## Day to day

```sh
journalctl -u pi-display -f  # watch the logs
pi-display version           # show the installed release
sudo pi-display update       # install the latest release, keeping your settings
pi-display on 2h             # stay on outside the schedule (see below)
```

`update` downloads the build for your Pi, checks it against the release's `SHA256SUMS`, swaps it in, and restarts the slideshow.

## Releasing

Push a version tag. GitHub Actions runs the tests, builds `arm64` and `armhf` binaries, and publishes a release with checksums.

```sh
git tag v0.1.0
git push origin v0.1.0
```

To build from source instead: `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o pi-display .`

## How it works

- SMB and Drive folders are mirrored into `/var/lib/pi-display/media` every 30 seconds. Playback runs from that copy, so the slideshow starts at boot before the network is up and keeps going if the source goes offline. Make sure the SD card has room for the whole folder.
- Drive is read through its public folder view and download links, not the API, so Google can break it by changing those pages. Native Google Docs are ignored. If two files share a name, the first one listed wins.
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

# Daily on/off times in the Pi's local time, 24-hour HH:MM (default: always on).
# An off time earlier than the on time runs past midnight.
[schedule]
on = "07:00"
off = "22:00"
```

The file is parsed strictly. If it has an unknown key, a wrong type, or a value out of range, the whole file is ignored, the last good settings stay in effect, and the error is logged. Nothing in it is passed to mpv as raw options.

## Staying on for an event

Outside the schedule the screen shows black. To keep the slideshow running anyway, SSH in and run:

```sh
pi-display on        # on for 1 hour
pi-display on 3h30m  # on for any duration
pi-display on 0      # cancel and go back to the schedule
```

It takes effect within 5 seconds and survives a reboot. Schedule changes and expired overrides take effect at the next slide. Set the Pi's timezone with `sudo raspi-config` so the schedule matches the wall clock.

## Files on the Pi

| Path | What |
| --- | --- |
| `/etc/pi-display/config.json` | Source and SMB credentials (0600) |
| `/usr/local/bin/pi-display` | Installed binary |
| `/etc/systemd/system/pi-display.service` | Boot service |
| `/var/lib/pi-display/media` | Mirrored SMB or Drive files |
| `/var/lib/pi-display/awake-until` | End time of a `pi-display on` override |
