import { useState } from "react";
import {
  cleanDates,
  dateLabel,
  dayBar,
  eventToml,
  fileName,
  isComplete,
  isUpcoming,
  isValidFolder,
  nextDay,
  today,
  type EventSettings,
} from "../lib/event";
import { isValidWindow } from "../lib/setup";
import { CopyButton, Field, RemoveButton, SecondsInput, Segmented, TimeInput, download, input, primaryButton } from "./wizard";

// The event page: pick a subfolder, dates, and hours, then download a
// <folder>.event.toml for the media folder. event.go reads it.

const initial: EventSettings = {
  folder: "",
  dates: [today()],
  allDay: false,
  hours: [{ on: "17:00", off: "22:00" }],
  imageSeconds: NaN,
  fit: "inherit",
  shuffle: "inherit",
  sound: "inherit",
};

const addButton = "h-9 self-start text-sm font-medium hover:text-muted";

export default function EventGenerator() {
  const [s, setS] = useState<EventSettings>(initial);
  const set = (patch: Partial<EventSettings>) => setS((prev) => ({ ...prev, ...patch }));
  const toml = eventToml(s);
  const name = fileName(s);
  const ready = isComplete(s);
  const folderInvalid = s.folder !== "" && !isValidFolder(s.folder);
  const dates = cleanDates(s.dates);

  return (
    <div className="flex flex-col items-start gap-14 lg:flex-row lg:gap-24">
      <div className="flex w-full flex-col gap-10 lg:w-[600px] lg:shrink-0">
        <Field label="Folder" hint="The subfolder of your media folder with the event's images and videos.">
          {(id) => (
            <input
              id={id}
              className={`${input} font-mono text-sm`}
              placeholder="gala"
              value={s.folder}
              aria-invalid={folderInvalid}
              onChange={(e) => set({ folder: e.target.value })}
              onBlur={() => set({ folder: s.folder.trim() })}
              spellCheck={false}
              autoComplete="off"
            />
          )}
        </Field>

        <div className="flex flex-col gap-2">
          <span id="dates-label" className="text-sm text-muted">
            Dates
          </span>
          <ul aria-labelledby="dates-label" className="flex flex-col gap-2">
            {s.dates.map((date, i) => (
              <li key={i} className="flex items-center gap-2">
                <input
                  aria-label={`Date ${i + 1}`}
                  type="date"
                  min={today()}
                  className={`${input} [color-scheme:dark] h-9 flex-1`}
                  value={date}
                  aria-invalid={!isUpcoming(date)}
                  onChange={(e) => set({ dates: s.dates.map((d, j) => (j === i ? e.target.value : d)) })}
                />
                {s.dates.length > 1 && (
                  <RemoveButton label={`Remove ${date}`} onClick={() => set({ dates: s.dates.filter((_, j) => j !== i) })} />
                )}
              </li>
            ))}
          </ul>
          <button
            type="button"
            className={addButton}
            onClick={() => {
              const last = [...s.dates].filter(isUpcoming).sort().at(-1);
              set({ dates: [...s.dates, last ? nextDay(last) : today()] });
            }}
          >
            + Another day
          </button>
          <p className="text-[13px] leading-5 text-faint">On these dates the event's hours replace the schedule. Past dates can't be picked.</p>
        </div>

        <div className="flex flex-col gap-2">
          <span id="hours-label" className="text-sm text-muted">
            Hours
          </span>
          <Segmented
            labelledBy="hours-label"
            options={[
              ["hours", "Set hours"],
              ["all", "Whole day"],
            ]}
            value={s.allDay ? "all" : "hours"}
            onChange={(v) => set({ allDay: v === "all" })}
          />
          {!s.allDay && (
            <>
              <ul className="mt-2 flex flex-col gap-2">
                {s.hours.map((w, i) => {
                  const change = (patch: Partial<typeof w>) => set({ hours: s.hours.map((h, j) => (j === i ? { ...h, ...patch } : h)) });
                  return (
                    <li key={i} className="flex items-center gap-2">
                      <TimeInput label={`Start ${i + 1}`} value={w.on} invalid={!isValidWindow(w)} onChange={(on) => change({ on })} compact />
                      <span className="text-sm text-faint">to</span>
                      <TimeInput label={`End ${i + 1}`} value={w.off} invalid={!isValidWindow(w)} onChange={(off) => change({ off })} compact />
                      {s.hours.length > 1 && (
                        <RemoveButton label={`Remove ${w.on}`} onClick={() => set({ hours: s.hours.filter((_, j) => j !== i) })} />
                      )}
                    </li>
                  );
                })}
              </ul>
              <button
                type="button"
                className={addButton}
                onClick={() => {
                  const last = s.hours.at(-1)?.off ?? "09:00";
                  set({ hours: [...s.hours, { on: last, off: last < "22:00" ? "22:00" : "23:59" }] });
                }}
              >
                + More hours
              </button>
              <p className="text-[13px] leading-5 text-faint">An end earlier than the start runs past midnight into the next day.</p>
            </>
          )}
        </div>

        <details className="group border-t border-line pt-8">
          <summary className="flex cursor-pointer list-none items-center justify-between text-[15px] font-semibold">
            Event settings
            <span className="text-sm font-normal text-muted group-open:hidden">Same as display.toml</span>
          </summary>
          <div className="mt-6 flex flex-col gap-6">
            <Field label="Seconds per image" hint="Leave blank to keep display.toml's.">
              {(id) => (
                <SecondsInput id={id} value={s.imageSeconds} onChange={(imageSeconds) => set({ imageSeconds })} optional />
              )}
            </Field>
            <Choice
              id="fit"
              label="Fit"
              value={s.fit}
              options={[
                ["contain", "Letterbox"],
                ["cover", "Fill and crop"],
              ]}
              onChange={(fit) => set({ fit })}
            />
            <Choice
              id="shuffle"
              label="Order"
              value={s.shuffle}
              options={[
                ["off", "By name"],
                ["on", "Shuffle"],
              ]}
              onChange={(shuffle) => set({ shuffle })}
            />
            <Choice
              id="sound"
              label="Video sound"
              value={s.sound}
              options={[
                ["muted", "Muted"],
                ["on", "On"],
              ]}
              onChange={(sound) => set({ sound })}
            />
          </div>
        </details>

        <div className="flex flex-col gap-5 border-t border-line pt-8">
          <h2 className="text-[15px] font-semibold">What happens</h2>
          {dates.length > 0 && (
            <div className="flex flex-col gap-4">
              {dates.slice(0, 7).map((date) => (
                <DayBar key={date} label={dateLabel(date)} {...dayBar(s, date, dates)} />
              ))}
              {dates.length > 7 && <p className="text-[13px] text-faint">and {dates.length - 7} more days</p>}
              <div className="flex justify-between pr-[84px] pl-[108px] font-mono text-[11px] text-faint">
                <span>00:00</span>
                <span className="hidden sm:inline">06:00</span>
                <span>12:00</span>
                <span className="hidden sm:inline">18:00</span>
                <span>24:00</span>
              </div>
            </div>
          )}
          <ul className="border-t border-line">
            {[
              ["Replaces the day", "During its hours every screen plays the folder. The rest of those dates the screens are off."],
              ["Same start everywhere", "Every screen starts on the folder's first file at the start time and stays in step."],
              ["Runs once", "After the last day it does nothing, and the Pis delete their copy of the folder."],
            ].map(([title, text]) => (
              <li key={title} className="flex flex-col gap-1 border-b border-line py-3.5 text-sm leading-[22px] sm:flex-row sm:gap-6">
                <span className="shrink-0 font-medium sm:w-[160px]">{title}</span>
                <span className="text-muted">{text}</span>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <div className="flex w-full min-w-0 flex-1 flex-col gap-5">
        <div className="rounded-lg border border-edge">
          <div className="flex h-12 items-center justify-between border-b border-line px-4 text-[13px] text-muted">
            <span className="font-mono">{name}</span>
            <span>{dates.length === 1 ? "1 day" : `${dates.length} days`}</span>
          </div>
          <div className="overflow-x-auto rounded-b-lg bg-screen p-6">
            <pre className="font-mono text-sm leading-6">{toml}</pre>
          </div>
        </div>
        <div className="flex items-center gap-2.5">
          <button type="button" disabled={!ready} className={`${primaryButton} gap-2`} onClick={() => download(name, toml)}>
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
              <path d="M7 1.5 V9.5 M3.5 6 L7 9.5 L10.5 6 M2 12.5 H12" stroke="currentColor" strokeWidth="1.5" />
            </svg>
            Download {name}
          </button>
          {ready && <CopyButton text={toml} secondary />}
        </div>
        {!ready && (
          <p className="text-[13px] text-faint">
            {!isValidFolder(s.folder) ? "Name the folder to play." : "Pick upcoming dates and hours that start and end at different times."}
          </p>
        )}

        <div className="flex flex-col gap-3.5 pt-4">
          <h2 className="text-[15px] font-semibold">Put it next to the folder</h2>
          <pre className="rounded-lg border border-line p-4 font-mono text-[13px] leading-6 text-muted">
            <span className="text-faint">Lobby screens/</span>
            {"\n  display.toml\n  "}
            <span className="text-fg">{name}</span> <span className="text-accent">← new</span>
            {"\n  "}
            <span className="text-fg">{isValidFolder(s.folder) ? s.folder : "…"}/</span>
            {"  "}
            <span className="text-faint">the event's images and videos</span>
            {"\n  welcome.jpg\n  menu.mp4"}
          </pre>
          <p className="text-sm leading-[22px] text-muted">
            Add both a day or so early. SMB and Drive Pis copy the folder as soon as the file appears, so everything is ready when the
            event starts. On Google Drive, upload the .toml file itself.{" "}
            <a href="/docs/events" className="text-fg underline decoration-edge underline-offset-4 hover:decoration-fg">
              More on events
            </a>
          </p>
        </div>
      </div>
    </div>
  );
}

// A setting that keeps display.toml's value unless one of options is picked.
function Choice<T extends string>({
  id,
  label,
  value,
  options,
  onChange,
}: {
  id: string;
  label: string;
  value: "inherit" | T;
  options: [T, string][];
  onChange: (value: "inherit" | T) => void;
}) {
  return (
    <div className="flex flex-col gap-2">
      <span id={`${id}-label`} className="text-sm text-muted">
        {label}
      </span>
      <Segmented labelledBy={`${id}-label`} options={[["inherit", "display.toml"], ...options]} value={value} onChange={onChange} />
    </div>
  );
}

// One date's 24 hours: white where the event plays, dark where it's off.
function DayBar({ label, on, spill }: { label: string; on: [number, number][]; spill: string | null }) {
  return (
    <div className="flex items-center gap-3">
      <span className="w-24 shrink-0 font-mono text-[13px] whitespace-nowrap text-muted">{label}</span>
      <div className="relative h-2 flex-1 rounded bg-line">
        {on.map(([a, b]) => (
          <div key={`${a}-${b}`} className="absolute inset-y-0 rounded bg-fg" style={{ left: `${(a / 1440) * 100}%`, width: `${((b - a) / 1440) * 100}%` }} />
        ))}
      </div>
      <span className="w-[72px] shrink-0 text-right font-mono text-[11px] text-accent">{spill && `→ ${spill}`}</span>
    </div>
  );
}

