// Builds an event file (event.go): a subfolder that plays on set dates and
// hours, once. The Pi rejects the whole file on any bad value, so this only
// emits what parseEvent accepts.

import { isValidSeconds, isValidWindow, type Fit, type Window } from "./setup";

export type Inherit<T extends string> = "inherit" | T;

export interface EventSettings {
  folder: string;
  dates: string[]; // YYYY-MM-DD
  allDay: boolean;
  hours: Window[];
  // Blank (NaN) keeps display.toml's value.
  imageSeconds: number;
  fit: Inherit<Fit>;
  shuffle: Inherit<"off" | "on">;
  sound: Inherit<"muted" | "on">;
}

// Today in the browser's local time, as YYYY-MM-DD.
export function today(): string {
  const d = new Date();
  return [d.getFullYear(), d.getMonth() + 1, d.getDate()].map((n) => String(n).padStart(2, "0")).join("-");
}

// The day after a YYYY-MM-DD date.
export function nextDay(date: string): string {
  const [y, m, d] = date.split("-").map(Number);
  const next = new Date(y!, m! - 1, d! + 1);
  return [next.getFullYear(), next.getMonth() + 1, next.getDate()].map((n) => String(n).padStart(2, "0")).join("-");
}

// Same rule as safeName in event.go: one visible folder name.
export const isValidFolder = (name: string) => name !== "" && !/[/\\]/.test(name) && !name.startsWith(".");

const isDate = (date: string) => /^\d{4}-\d{2}-\d{2}$/.test(date);

export const isUpcoming = (date: string) => isDate(date) && date >= today();

// The dates to write: valid, not past, sorted, once each.
export const cleanDates = (dates: string[]) => [...new Set(dates.filter(isUpcoming))].sort();

export const fileName = (s: EventSettings) => `${isValidFolder(s.folder) ? s.folder : "…"}.event.toml`;

// Whether the file is ready: a folder, at least one date, and good hours.
export function isComplete(s: EventSettings): boolean {
  return (
    isValidFolder(s.folder) &&
    s.dates.every(isUpcoming) &&
    cleanDates(s.dates).length > 0 &&
    (s.allDay || (s.hours.length > 0 && s.hours.every(isValidWindow))) &&
    (Number.isNaN(s.imageSeconds) || isValidSeconds(s.imageSeconds))
  );
}

const q = (text: string) => JSON.stringify(text);

export function eventToml(s: EventSettings): string {
  const lines = [
    `folder = ${isValidFolder(s.folder) ? q(s.folder) : '"…"'}`,
    `dates = [${cleanDates(s.dates).map(q).join(", ")}]`,
  ];
  if (s.allDay) lines.push("# no hours: plays all day");
  else lines.push(`hours = [${s.hours.filter(isValidWindow).map((w) => q(`${w.on}-${w.off}`)).join(", ")}]`);

  const settings = [];
  if (isValidSeconds(s.imageSeconds)) settings.push(`image_seconds = ${s.imageSeconds}`);
  if (s.fit !== "inherit") settings.push(`fit = ${q(s.fit)}`);
  if (s.shuffle !== "inherit") settings.push(`shuffle = ${s.shuffle === "on"}`);
  if (s.sound !== "inherit") settings.push(`mute = ${s.sound === "muted"}`);
  if (settings.length > 0) lines.push("", "# for this event only", ...settings);
  return lines.join("\n") + "\n";
}

const minutes = (time: string) => {
  const [h, m] = time.split(":").map(Number);
  return h! * 60 + m!;
};

// The stretches of a date that are on, in minutes after midnight: its own
// windows up to midnight, plus the overnight tail of the day before's when
// that's an event date too. spill is the time an overnight window ends the
// next morning when the next day isn't an event date.
export function dayBar(s: EventSettings, date: string, dates: string[]): { on: [number, number][]; spill: string | null } {
  if (s.allDay) return { on: [[0, 1440]], spill: null };
  const windows = s.hours.filter(isValidWindow);
  const on: [number, number][] = [];
  let spill: string | null = null;
  for (const w of windows) {
    const [a, b] = [minutes(w.on), minutes(w.off)];
    if (b > a) on.push([a, b]);
    else {
      on.push([a, 1440]);
      if (!dates.includes(nextDay(date))) spill = w.off;
    }
  }
  if (dates.some((d) => nextDay(d) === date)) {
    for (const w of windows) {
      const [a, b] = [minutes(w.on), minutes(w.off)];
      if (b < a) on.push([0, b]);
    }
  }
  return { on, spill };
}

// "Sat, Dec 5" for a YYYY-MM-DD date.
export function dateLabel(date: string): string {
  const [y, m, d] = date.split("-").map(Number);
  return new Date(y!, m! - 1, d!).toLocaleDateString("en-US", { weekday: "short", month: "short", day: "numeric" });
}
