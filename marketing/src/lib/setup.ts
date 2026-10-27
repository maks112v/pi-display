// Builds the one-line setup command the config generator hands to the user.
// The code is a Device (config.go) as JSON, base64-encoded, carrying the
// source, login, and slideshow settings. The Pi rejects the whole code on any
// unknown key or out-of-range value, so this only emits values that pass.

export type Fit = "contain" | "cover";

// Where the media lives. Picked first in the generator; null until chosen.
export type SourceKind = "smb" | "drive";

// A stretch of on time as "HH:MM". Off earlier than on runs past midnight.
export interface Window {
  on: string;
  off: string;
}

// Same keys and order as dayKeys in config.go.
export const days = ["sun", "mon", "tue", "wed", "thu", "fri", "sat"] as const;
export type Day = (typeof days)[number];
export const dayNames: Record<Day, string> = {
  sun: "Sunday",
  mon: "Monday",
  tue: "Tuesday",
  wed: "Wednesday",
  thu: "Thursday",
  fri: "Friday",
  sat: "Saturday",
};

// "always" plays nonstop, "daily" uses one window for every day, and "weekly"
// uses a list of windows per day. Both are kept so switching modes doesn't
// lose what was typed.
export interface Schedule {
  mode: "always" | "daily" | "weekly";
  daily: Window;
  weekly: Record<Day, Window[]>;
}

export interface Override {
  name: string;
  seconds: number;
}

export interface Settings {
  kind: SourceKind | null;
  source: string;
  username: string;
  password: string;
  imageSeconds: number;
  fit: Fit;
  mute: boolean;
  shuffle: boolean;
  overrides: Override[];
  schedule: Schedule;
}

export const defaultSettings: Settings = {
  kind: null,
  source: "",
  username: "",
  password: "",
  imageSeconds: 10,
  fit: "contain",
  mute: true,
  shuffle: false,
  overrides: [],
  schedule: { mode: "always", daily: { on: "07:00", off: "22:00" }, weekly: { sun: [], mon: [], tue: [], wed: [], thu: [], fri: [], sat: [] } },
};

// Downloads the latest release for the Pi's architecture (arm64 or armhf)
// and makes it executable. Matches step 2 of the README.
export const installCommand = [
  `curl -fsSLo pi-display "https://github.com/maks112v/pi-display/releases/latest/download/pi-display-$(dpkg --print-architecture)"`,
  "chmod +x pi-display",
].join("\n");

// Same bound as maxSeconds in config.go.
export const maxSeconds = 24 * 60 * 60;

export const isValidSeconds = (n: number) => Number.isInteger(n) && n >= 1 && n <= maxSeconds;

export const isValidWindow = (w: Window) => w.on !== "" && w.off !== "" && w.on !== w.off;

// The schedule as config.go's Schedule reads it, or null for always on (or a
// weekly schedule with no usable hours yet, which the generator blocks).
export function scheduleJSON(s: Schedule): Record<string, string | string[]> | null {
  if (s.mode === "daily") return isValidWindow(s.daily) ? { ...s.daily } : null;
  if (s.mode === "always") return null;
  const entries = days.flatMap((day) => {
    const windows = s.weekly[day].filter(isValidWindow).map((w) => `${w.on}-${w.off}`);
    return windows.length > 0 ? [[day, windows] as const] : [];
  });
  return entries.length > 0 ? Object.fromEntries(entries) : null;
}

// Same check as parseDriveFolder in drive.go.
function isDriveFolder(source: string) {
  if (!source.startsWith("https://drive.google.com/")) return false;
  const id = new URL(source).pathname.split("/folders/")[1]?.split("/")[0] ?? "";
  return /^[\w-]{10,}$/.test(id);
}

// The source as the Pi expects it, or null if it can't be used yet. SMB
// addresses may skip the smb:// prefix or use Windows form (\\nas\screens).
export function normalizeSource(kind: SourceKind | null, raw: string): string | null {
  const source = raw.trim();
  if (!kind || !source) return null;
  if (kind === "drive") return isDriveFolder(source) ? source : null;
  const path = source.replace(/^smb:\/\//i, "").replaceAll("\\", "/").replace(/^\/+/, "");
  const [host, share] = path.split("/");
  return host && share ? `smb://${path}` : null;
}

// Mirrors the JSON shape of Device and Show in config.go.
function device(s: Settings, source: string) {
  const login = s.kind === "smb"
    ? { ...(s.username && { username: s.username }), ...(s.username && s.password && { password: s.password }) }
    : {};

  const durations = Object.fromEntries(
    s.overrides.filter((o) => o.name.trim() && isValidSeconds(o.seconds)).map((o) => [o.name.trim(), o.seconds]),
  );
  const schedule = scheduleJSON(s.schedule);

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
  const source = normalizeSource(s.kind, s.source);
  if (!source) return null;
  const bytes = new TextEncoder().encode(JSON.stringify(device(s, source)));
  const code = btoa(Array.from(bytes, (b) => String.fromCharCode(b)).join(""));
  return `sudo ./pi-display setup ${code}`;
}
