// Builds the one-line setup command the config generator hands to the user.
// The code is a Device (config.go) as JSON, base64-encoded, carrying the
// source, login, and slideshow settings. The Pi rejects the whole code on any
// unknown key or out-of-range value, so this only emits values that pass.

export type Fit = "contain" | "cover";

export interface Override {
  name: string;
  seconds: number;
}

export interface Settings {
  source: string;
  username: string;
  password: string;
  imageSeconds: number;
  fit: Fit;
  mute: boolean;
  shuffle: boolean;
  overrides: Override[];
  // Daily on/off window as "HH:MM", or null for always on.
  schedule: { on: string; off: string } | null;
}

export const defaultSettings: Settings = {
  source: "",
  username: "",
  password: "",
  imageSeconds: 10,
  fit: "contain",
  mute: true,
  shuffle: false,
  overrides: [],
  schedule: null,
};

// Same bound as maxSeconds in config.go.
export const maxSeconds = 24 * 60 * 60;

export const isValidSeconds = (n: number) => Number.isInteger(n) && n >= 1 && n <= maxSeconds;

// Only SMB shares take a login; Drive links and local paths ignore it.
export const needsLogin = (source: string) => source.trim().toLowerCase().startsWith("smb://");

// Mirrors the JSON shape of Device and Show in config.go.
function device(s: Settings) {
  const source = s.source.trim();
  const login = needsLogin(source)
    ? { ...(s.username && { username: s.username }), ...(s.username && s.password && { password: s.password }) }
    : {};

  const durations = Object.fromEntries(
    s.overrides.filter((o) => o.name.trim() && isValidSeconds(o.seconds)).map((o) => [o.name.trim(), o.seconds]),
  );
  const schedule = s.schedule && s.schedule.on && s.schedule.off && s.schedule.on !== s.schedule.off ? s.schedule : null;

  return {
    source,
    ...login,
    show: {
      image_seconds: isValidSeconds(s.imageSeconds) ? s.imageSeconds : defaultSettings.imageSeconds,
      mute: s.mute,
      fit: s.fit,
      shuffle: s.shuffle,
      ...(Object.keys(durations).length > 0 && { durations }),
      ...(schedule && { schedule }),
    },
  };
}

export function setupCommand(s: Settings): string | null {
  if (!s.source.trim()) return null;
  const bytes = new TextEncoder().encode(JSON.stringify(device(s)));
  const code = btoa(Array.from(bytes, (b) => String.fromCharCode(b)).join(""));
  return `sudo ./pi-display setup ${code}`;
}
