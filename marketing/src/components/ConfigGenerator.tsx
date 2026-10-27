import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import {
  dayNames,
  days,
  defaultSettings,
  installCommand,
  isValidSeconds,
  isValidWindow,
  maxSeconds,
  normalizeSource,
  scheduleJSON,
  setupCommand,
  type Day,
  type Schedule,
  type Settings,
  type SourceKind,
  type Window,
} from "../lib/setup";

const input =
  "h-11 w-full rounded-md border border-edge bg-transparent px-3.5 text-[15px] outline-none placeholder:text-faint focus:border-fg aria-invalid:border-red-500";

const steps = ["Install", "Media source", "Playback", "Schedule", "Run setup"] as const;

// Guided setup: install the binary, pick a media source, tune playback, pick
// the hours, then run the generated setup command. Nothing leaves the browser:
// the command is computed from local state on render.
export default function ConfigGenerator() {
  const [settings, setSettings] = useState<Settings>(defaultSettings);
  const update = (patch: Partial<Settings>) => setSettings((s) => ({ ...s, ...patch }));
  const [step, setStep] = useState(0);
  // Furthest step reached, so finished steps in the list stay clickable.
  const [reached, setReached] = useState(0);

  const source = normalizeSource(settings.kind, settings.source);
  const command = setupCommand(settings);
  // Only flag the address once something is typed, so an empty field isn't red.
  const sourceInvalid = settings.source.trim() !== "" && !source;
  const schedule = scheduleJSON(settings.schedule);
  // Steps past the media source need a usable one, and a timed schedule needs
  // at least one window.
  const canLeave =
    (step !== 1 || source !== null) && (step !== 3 || settings.schedule.mode === "always" || schedule !== null);

  const heading = useRef<HTMLHeadingElement>(null);
  const mounted = useRef(false);
  useEffect(() => {
    // Move focus (and scroll) to the new step, but not on first load.
    if (mounted.current) heading.current?.focus();
    mounted.current = true;
  }, [step]);

  const go = (next: number) => {
    setStep(next);
    setReached((r) => Math.max(r, next));
  };

  const summaries: (string | null)[] = [
    null,
    settings.kind === "drive" ? "Google Drive folder" : source?.replace(/^smb:\/\//, "") ?? null,
    `${isValidSeconds(settings.imageSeconds) ? settings.imageSeconds : defaultSettings.imageSeconds}s per image`,
    !schedule
      ? "Always on"
      : settings.schedule.mode === "daily"
        ? `${settings.schedule.daily.on}-${settings.schedule.daily.off} daily`
        : days
            .filter((day) => day in schedule)
            .map((day) => dayNames[day].slice(0, 3))
            .join(", "),
    null,
  ];

  return (
    <div className="grid items-start gap-12 lg:grid-cols-[224px_minmax(0,720px)] lg:gap-24">
      <ol className="hidden border-l border-line lg:flex lg:flex-col">
        {steps.map((label, i) => {
          const current = i === step;
          const done = i < reached && !current && (i !== 1 || source !== null);
          const enabled = !current && i <= reached && (i <= 1 || source !== null);
          return (
            <li key={label}>
              <button
                type="button"
                disabled={!enabled}
                aria-current={current ? "step" : undefined}
                onClick={() => go(i)}
                className="-ml-px flex w-full items-center gap-3.5 border-l-2 border-transparent py-2.5 pl-5 text-left text-[15px] text-muted enabled:hover:text-fg aria-[current=step]:border-accent aria-[current=step]:font-medium aria-[current=step]:text-fg"
              >
                <span className="flex w-4 shrink-0 justify-center font-mono text-[13px] text-faint">
                  {current ? (
                    <span className="text-accent">{i + 1}</span>
                  ) : done ? (
                    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-label="Done" className="text-muted">
                      <path d="M2.5 7.5 L5.5 10.5 L11.5 3.5" stroke="currentColor" strokeWidth="1.5" />
                    </svg>
                  ) : (
                    i + 1
                  )}
                </span>
                <span className="flex min-w-0 flex-col gap-0.5">
                  {label}
                  {done && summaries[i] && (
                    <span className="truncate font-mono text-xs font-normal text-faint">{summaries[i]}</span>
                  )}
                </span>
              </button>
            </li>
          );
        })}
      </ol>

      <form
        className="flex min-w-0 flex-col gap-10"
        onSubmit={(e) => {
          e.preventDefault();
          if (canLeave && step < steps.length - 1) go(step + 1);
        }}
      >
        <div className="flex flex-col gap-2">
          <span className="font-mono text-[13px] text-faint">
            Step {step + 1} of {steps.length}
          </span>
          <h2 ref={heading} tabIndex={-1} className="text-2xl font-semibold tracking-[-0.02em] outline-none">
            {["Install pi-display", "Where is your media?", "How should it play?", "When should it play?", "Run setup"][step]}
          </h2>
          <p className="text-[15px] leading-6 text-muted">
            {
              [
                "SSH into the Pi and run these two commands. New Pi? Flash Raspberry Pi OS Lite with SSH turned on first.",
                "The Pi copies every image and video from this folder and keeps it in sync.",
                "The defaults work for most screens. You can change any of this later.",
                "Outside these hours the screen shows black. Times are the Pi's local time.",
                "Run this in the same folder as before. It carries everything you picked, so there are no questions to answer.",
              ][step]
            }
          </p>
        </div>

        {step === 0 && (
          <>
            <Output label="Terminal on the Pi" action={<CopyButton text={installCommand} />}>
              <pre className="font-mono text-sm leading-6 whitespace-pre-wrap [overflow-wrap:anywhere]">{installCommand}</pre>
            </Output>
            <Explain
              title="What these do"
              rows={[
                ["curl -fsSLo pi-display", "Downloads the latest release from GitHub and saves it as a file named pi-display."],
                ["$(dpkg --print-arch…)", "Asks the Pi whether it runs 64-bit (arm64) or 32-bit (armhf), so you get the matching build."],
                ["chmod +x pi-display", "Marks the file as a program so the Pi is allowed to run it."],
              ]}
            />
          </>
        )}

        {step === 1 && (
          <>
            <SourcePicker value={settings.kind} onChange={(kind) => update({ kind, source: "" })} />
            {settings.kind === "smb" && (
              <div className="flex flex-col gap-5">
                <Field label="Share address" hint="Like smb://nas.local/screens or \\nas\screens">
                  {(id) => (
                    <input
                      id={id}
                      className={`${input} font-mono text-sm`}
                      placeholder="smb://nas.local/screens"
                      value={settings.source}
                      aria-invalid={sourceInvalid}
                      onChange={(e) => update({ source: e.target.value })}
                      autoComplete="off"
                      spellCheck={false}
                      autoFocus
                    />
                  )}
                </Field>
                <div className="flex gap-4">
                  <Field label="Username">
                    {(id) => (
                      <input
                        id={id}
                        className={input}
                        value={settings.username}
                        onChange={(e) => update({ username: e.target.value })}
                        autoComplete="off"
                      />
                    )}
                  </Field>
                  <Field label="Password">
                    {(id) => (
                      <input
                        id={id}
                        type="password"
                        className={input}
                        value={settings.password}
                        onChange={(e) => update({ password: e.target.value })}
                        autoComplete="new-password"
                      />
                    )}
                  </Field>
                </div>
                <p className="text-[13px] leading-5 text-faint">
                  Leave both empty for a guest share. To have each screen write its log to the share, the login needs write
                  access.
                </p>
              </div>
            )}
            {settings.kind === "drive" && (
              <div className="flex flex-col gap-5">
                <Field label="Folder link">
                  {(id) => (
                    <input
                      id={id}
                      className={`${input} font-mono text-sm`}
                      placeholder="https://drive.google.com/drive/folders/..."
                      value={settings.source}
                      aria-invalid={sourceInvalid}
                      onChange={(e) => update({ source: e.target.value })}
                      autoComplete="off"
                      spellCheck={false}
                      autoFocus
                    />
                  )}
                </Field>
                <div className="flex flex-col gap-2.5 border-y border-line py-4">
                  <span className="text-sm font-medium">Get the link</span>
                  <ol className="list-inside list-decimal text-sm leading-[22px] text-muted">
                    <li>In Drive, right-click the folder and choose Share.</li>
                    <li>Under General access, pick Anyone with the link.</li>
                    <li>Copy link and paste it above.</li>
                  </ol>
                </div>
                <p className="text-[13px] leading-5 text-faint">
                  Only files at the top of the folder play. Native Google Docs are skipped.
                </p>
              </div>
            )}
          </>
        )}

        {step === 2 && (
          <>
            <div className="flex flex-col gap-5">
              <div className="flex gap-4">
                <div className="w-40 shrink-0">
                  <Field label="Seconds per image">
                    {(id) => (
                      <SecondsInput id={id} value={settings.imageSeconds} onChange={(imageSeconds) => update({ imageSeconds })} />
                    )}
                  </Field>
                </div>
                <div className="flex min-w-0 flex-1 flex-col gap-2">
                  <span id="fit-label" className="text-sm text-muted">
                    Fit
                  </span>
                  <Segmented
                    labelledBy="fit-label"
                    options={[
                      ["contain", "Contain"],
                      ["cover", "Cover"],
                    ]}
                    value={settings.fit}
                    onChange={(fit) => update({ fit })}
                  />
                </div>
              </div>
              <Switch label="Mute video audio" checked={settings.mute} onChange={(mute) => update({ mute })} />
              <Switch label="Shuffle the order" checked={settings.shuffle} onChange={(shuffle) => update({ shuffle })} />
            </div>

            <div className="flex flex-col gap-3 border-t border-line pt-8">
              <div className="flex items-baseline justify-between">
                <span className="text-[15px] font-semibold">Longer slides</span>
                <span className="text-[13px] text-faint">Optional</span>
              </div>
              {settings.overrides.length > 0 && (
                <ul className="border-b border-line">
                  {settings.overrides.map((override, i) => {
                    const set = (patch: Partial<typeof override>) =>
                      update({ overrides: settings.overrides.map((o, j) => (j === i ? { ...o, ...patch } : o)) });
                    return (
                      <li key={i} className="flex items-center gap-3 border-t border-line py-2">
                        <input
                          aria-label="File name"
                          className="h-9 min-w-0 flex-1 bg-transparent font-mono text-sm outline-none placeholder:text-faint"
                          placeholder="intro.jpg"
                          value={override.name}
                          onChange={(e) => set({ name: e.target.value })}
                          spellCheck={false}
                        />
                        <div className="w-24 shrink-0">
                          <SecondsInput label="Seconds" value={override.seconds} onChange={(seconds) => set({ seconds })} compact />
                        </div>
                        <RemoveButton
                          label={`Remove ${override.name || "file"}`}
                          onClick={() => update({ overrides: settings.overrides.filter((_, j) => j !== i) })}
                        />
                      </li>
                    );
                  })}
                </ul>
              )}
              <button
                type="button"
                className="self-start text-sm font-medium hover:text-muted"
                onClick={() => update({ overrides: [...settings.overrides, { name: "", seconds: 30 }] })}
              >
                + Add file
              </button>
            </div>
          </>
        )}

        {step === 3 && (
          <ScheduleEditor value={settings.schedule} onChange={(schedule) => update({ schedule })} />
        )}

        {step === 4 && command && (
          <>
            <div className="flex flex-col gap-3">
              <Output label="Terminal on the Pi" action={<CopyButton text={command} />}>
                <p className="font-mono text-sm leading-6 break-all">{command}</p>
              </Output>
              {settings.kind === "smb" && settings.password && (
                <p className="text-[13px] leading-5 text-faint">
                  The code includes your password, encoded but not encrypted. Share it only with people allowed to know it.
                </p>
              )}
            </div>
            <Explain
              title="What happens next"
              rows={[
                "Connects to your folder and counts the images and videos. A wrong link or password stops here.",
                "Installs mpv, the video player, if it is missing.",
                "Installs itself and starts the slideshow on every boot. You can delete the downloaded file afterwards.",
                "Boots to the console instead of a desktop, so the slideshow has the screen to itself.",
              ]}
            />
            <Explain
              title="Later"
              rows={[
                ["sudo pi-display update", "Installs the latest release and keeps your settings."],
                ["pi-display on 2h", "Keeps the screen on outside the schedule."],
              ]}
            />
          </>
        )}

        <div className="flex items-center justify-between">
          <button
            type="button"
            className="text-sm text-muted hover:text-fg disabled:invisible"
            disabled={step === 0}
            onClick={() => go(step - 1)}
          >
            Back
          </button>
          <div className="flex items-center gap-5">
            {step === 0 && (
              <button type="button" className="text-sm text-muted hover:text-fg" onClick={() => go(1)}>
                Already installed? Skip
              </button>
            )}
            {step < steps.length - 1 ? (
              <button
                type="submit"
                disabled={!canLeave}
                className="flex h-10 items-center rounded-md bg-fg px-[18px] text-sm font-medium text-bg hover:bg-muted disabled:bg-edge disabled:text-faint"
              >
                Continue
              </button>
            ) : (
              <button
                type="button"
                className="flex h-10 items-center rounded-md border border-edge px-[18px] text-sm font-medium hover:border-muted"
                onClick={() => {
                  setSettings((s) => ({ ...s, kind: null, source: "", username: "", password: "" }));
                  setReached(1);
                  go(1);
                }}
              >
                Set up another screen
              </button>
            )}
          </div>
        </div>
      </form>
    </div>
  );
}

// A two-column table: a command (or step number) beside a plain explanation.
// Plain-string rows are numbered instead, with a narrow, muted label column.
function Explain({ title, rows }: { title: string; rows: string[] | [string, string][] }) {
  return (
    <section className="flex flex-col gap-4">
      <h3 className="text-[15px] font-semibold">{title}</h3>
      <ul className="border-t border-line">
        {rows.map((row, i) => {
          const numbered = typeof row === "string";
          const [label, text] = numbered ? [String(i + 1), row] : row;
          return (
            <li key={i} className="flex flex-col gap-1 border-b border-line py-3.5 sm:flex-row sm:gap-6">
              <span className={`shrink-0 font-mono text-[13px] leading-[22px] ${numbered ? "w-6 text-faint" : "sm:w-[200px]"}`}>
                {label}
              </span>
              <span className="flex-1 text-sm leading-[22px] text-muted">{text}</span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

// Renders a label above its input; the input gets the generated id.
function Field({ label, hint, children }: { label: string; hint?: string; children: (id: string) => ReactNode }) {
  const id = useId();
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-2">
      <label htmlFor={id} className="text-sm text-muted">
        {label}
      </label>
      {children(id)}
      {hint && <p className="text-[13px] text-faint">{hint}</p>}
    </div>
  );
}

function SecondsInput({
  id,
  label,
  value,
  onChange,
  compact = false,
}: {
  id?: string;
  label?: string;
  value: number;
  onChange: (seconds: number) => void;
  compact?: boolean;
}) {
  return (
    <input
      id={id}
      aria-label={label}
      type="number"
      inputMode="numeric"
      min={1}
      max={maxSeconds}
      className={compact ? `${input} h-9 text-right` : input}
      value={Number.isNaN(value) ? "" : value}
      aria-invalid={!isValidSeconds(value)}
      onChange={(e) => onChange(e.target.valueAsNumber)}
    />
  );
}

const sourceOptions: { kind: SourceKind; label: string; detail: string; icon: ReactNode }[] = [
  {
    kind: "smb",
    label: "SMB share",
    detail: "A NAS or shared folder on your network",
    icon: (
      <>
        <rect x="2" y="3" width="18" height="6" rx="1.5" stroke="currentColor" strokeWidth="1.5" />
        <rect x="2" y="13" width="18" height="6" rx="1.5" stroke="currentColor" strokeWidth="1.5" />
        <circle cx="6" cy="6" r="1" fill="currentColor" />
        <circle cx="6" cy="16" r="1" fill="currentColor" />
      </>
    ),
  },
  {
    kind: "drive",
    label: "Google Drive",
    detail: "A folder shared with anyone who has the link",
    icon: (
      <>
        <path d="M8 3 H14 L20 14 L17 19 H5 L2 14 Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
        <path d="M8 3 L14 14 H20 M2 14 H8 L5 19" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
      </>
    ),
  },
];

function SourcePicker({ value, onChange }: { value: SourceKind | null; onChange: (kind: SourceKind) => void }) {
  return (
    <div role="radiogroup" aria-label="Media source" className="grid gap-3 sm:grid-cols-2">
      {sourceOptions.map((option) => (
        <button
          key={option.kind}
          type="button"
          role="radio"
          aria-checked={value === option.kind}
          className="group flex flex-col gap-3.5 rounded-lg border border-edge p-5 text-left hover:border-muted aria-checked:border-fg"
          onClick={() => value !== option.kind && onChange(option.kind)}
        >
          <svg width="22" height="22" viewBox="0 0 22 22" fill="none" aria-hidden="true" className="text-muted group-aria-checked:text-fg">
            {option.icon}
          </svg>
          <span className="flex flex-col gap-1">
            <span className="text-base font-semibold">{option.label}</span>
            <span className="text-sm leading-5 text-muted">{option.detail}</span>
          </span>
        </button>
      ))}
    </div>
  );
}

// A row of mutually exclusive options, as [value, label] pairs.
function Segmented<T extends string>({
  labelledBy,
  options,
  value,
  onChange,
}: {
  labelledBy: string;
  options: [T, string][];
  value: T;
  onChange: (value: T) => void;
}) {
  return (
    <div role="radiogroup" aria-labelledby={labelledBy} className="flex h-11 gap-1 rounded-md border border-edge p-1">
      {options.map(([option, label]) => (
        <button
          key={option}
          type="button"
          role="radio"
          aria-checked={value === option}
          className="flex-1 rounded text-sm text-muted aria-checked:bg-line aria-checked:text-fg"
          onClick={() => onChange(option)}
        >
          {label}
        </button>
      ))}
    </div>
  );
}

// Picks always on, one window every day, or windows per weekday. Each weekday
// takes any number of windows, so a day can have a morning and an evening.
function ScheduleEditor({ value, onChange }: { value: Schedule; onChange: (schedule: Schedule) => void }) {
  const setDay = (day: Day, windows: Window[]) => onChange({ ...value, weekly: { ...value.weekly, [day]: windows } });

  return (
    <div className="flex flex-col gap-8">
      <div className="flex flex-col gap-2">
        <span id="schedule-label" className="text-sm text-muted">
          Plays
        </span>
        <Segmented
          labelledBy="schedule-label"
          options={[
            ["always", "All the time"],
            ["daily", "Same hours daily"],
            ["weekly", "By day of week"],
          ]}
          value={value.mode}
          onChange={(mode) => onChange({ ...value, mode })}
        />
      </div>

      {value.mode === "daily" && (
        <div className="flex gap-4">
          {(["on", "off"] as const).map((key) => (
            <Field key={key} label={key === "on" ? "Screen on" : "Screen off"}>
              {(id) => (
                <TimeInput
                  id={id}
                  value={value.daily[key]}
                  onChange={(time) => onChange({ ...value, daily: { ...value.daily, [key]: time } })}
                />
              )}
            </Field>
          ))}
        </div>
      )}

      {value.mode === "weekly" && (
        <div className="flex flex-col gap-3">
          <ul className="border-b border-line">
            {days.map((day) => {
              const name = dayNames[day];
              const windows = value.weekly[day];
              return (
                <li key={day} className="flex items-start gap-4 border-t border-line py-2.5">
                  <span className="w-24 shrink-0 text-[15px] leading-9">{name}</span>
                  <div className="flex min-w-0 flex-1 flex-col gap-2">
                    {windows.length === 0 && <span className="text-sm leading-9 text-faint">Off</span>}
                    {windows.map((w, j) => {
                      const set = (patch: Partial<Window>) =>
                        setDay(day, windows.map((o, k) => (k === j ? { ...o, ...patch } : o)));
                      return (
                        <div key={j} className="flex items-center gap-2">
                          <TimeInput label={`${name} on`} value={w.on} invalid={!isValidWindow(w)} onChange={(on) => set({ on })} compact />
                          <span className="text-sm text-faint">to</span>
                          <TimeInput label={`${name} off`} value={w.off} invalid={!isValidWindow(w)} onChange={(off) => set({ off })} compact />
                          <RemoveButton
                            label={`Remove ${name} ${w.on}`}
                            onClick={() => setDay(day, windows.filter((_, k) => k !== j))}
                          />
                        </div>
                      );
                    })}
                  </div>
                  <button
                    type="button"
                    className="h-9 shrink-0 text-sm font-medium hover:text-muted"
                    onClick={() => setDay(day, [...windows, nextWindow(windows.at(-1))])}
                  >
                    + Hours
                  </button>
                </li>
              );
            })}
          </ul>
          <p className="text-[13px] leading-5 text-faint">
            An off time earlier than the on time runs past midnight into the next day.
          </p>
        </div>
      )}
    </div>
  );
}

// A new window for a day: 09:00 to 18:00, or after the day's last window.
function nextWindow(last: Window | undefined): Window {
  if (!last) return { on: "09:00", off: "18:00" };
  return { on: last.off, off: last.off < "22:00" ? "22:00" : "23:59" };
}

function TimeInput({
  id,
  label,
  value,
  invalid = false,
  onChange,
  compact = false,
}: {
  id?: string;
  label?: string;
  value: string;
  invalid?: boolean;
  onChange: (time: string) => void;
  compact?: boolean;
}) {
  return (
    <input
      id={id}
      aria-label={label}
      type="time"
      className={`${input} [color-scheme:dark] ${compact ? "h-9 w-auto min-w-0 flex-1" : ""}`}
      value={value}
      aria-invalid={invalid || value === ""}
      onChange={(e) => onChange(e.target.value)}
    />
  );
}

function RemoveButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      aria-label={label}
      className="flex size-8 shrink-0 items-center justify-center rounded text-faint hover:text-fg"
      onClick={onClick}
    >
      <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">
        <path d="M2 2 L10 10 M10 2 L2 10" stroke="currentColor" strokeWidth="1.5" />
      </svg>
    </button>
  );
}

function Switch({ label, checked, onChange }: { label: ReactNode; checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      className="group flex items-center justify-between text-left text-[15px]"
      onClick={() => onChange(!checked)}
    >
      {label}
      <span className="flex h-5 w-9 items-center rounded-full bg-edge p-0.5 group-aria-checked:justify-end group-aria-checked:bg-fg">
        <span className="size-4 rounded-full bg-fg group-aria-checked:bg-bg" />
      </span>
    </button>
  );
}

function Output({ label, action, children }: { label: ReactNode; action: ReactNode; children: ReactNode }) {
  return (
    <div className="rounded-lg border border-edge">
      <div className="flex h-12 items-center justify-between border-b border-line px-4 text-[13px] text-muted">
        {label}
        {action}
      </div>
      <div className="overflow-x-auto p-4">{children}</div>
    </div>
  );
}

const smallButton = "rounded-[5px] px-3 py-1.5 text-[13px] font-medium";

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  return (
    <button
      type="button"
      className={`${smallButton} bg-fg text-bg hover:bg-muted`}
      onClick={() => navigator.clipboard.writeText(text).then(() => setCopied(true))}
    >
      {copied ? "Copied" : "Copy"}
    </button>
  );
}
