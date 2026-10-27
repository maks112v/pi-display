import { useState } from "react";
import {
  configCommand,
  dayNames,
  days,
  defaultShow,
  displayToml,
  isValidSeconds,
  isValidWindow,
  scheduleJSON,
  type Day,
  type Schedule,
  type ShowSettings,
  type Window,
} from "../lib/setup";
import {
  CopyButton,
  Field,
  Output,
  SecondsInput,
  Segmented,
  Switch,
  Wizard,
  input,
  primaryButton,
  smallButton,
  useWizard,
  type WizardStep,
} from "./wizard";

// The config page: playback, schedule, then a display.toml to download for
// the media folder, or a command that applies the settings to one Pi.
// Nothing leaves the browser: both are computed from local state.
export default function ConfigWizard() {
  const [settings, setSettings] = useState<ShowSettings>(defaultShow);
  const update = (patch: Partial<ShowSettings>) => setSettings((s) => ({ ...s, ...patch }));
  const [onePi, setOnePi] = useState(false);
  const wizard = useWizard();
  const { step } = wizard;

  const schedule = scheduleJSON(settings.schedule);
  // A timed schedule needs at least one usable window before saving.
  const scheduleReady = settings.schedule.mode === "always" || schedule !== null;
  const toml = displayToml(settings);

  const steps: WizardStep[] = [
    {
      label: "Playback",
      title: "How should it play?",
      description: "The defaults work for most screens. Change only what you need.",
      summary: `${isValidSeconds(settings.imageSeconds) ? settings.imageSeconds : defaultShow.imageSeconds}s per image, ${settings.fit}`,
    },
    {
      label: "Schedule",
      title: "When should it be on?",
      description: "Outside these hours the screen shows black. Times are the Pi's local time.",
      summary: !schedule
        ? "Always on"
        : settings.schedule.mode === "daily"
          ? `${settings.schedule.daily.on}-${settings.schedule.daily.off} daily`
          : days
              .filter((day) => day in schedule)
              .map((day) => dayNames[day].slice(0, 3))
              .join(", "),
    },
    {
      label: "Save",
      title: "Save your config",
      description: "Put display.toml in your media folder and every screen playing from it picks it up within a slide.",
    },
  ];

  return (
    <Wizard
      {...wizard}
      steps={steps}
      canOpen={(i) => i <= 1 || scheduleReady}
      canLeave={step !== 1 || scheduleReady}
      footer={
        step < steps.length - 1 ? (
          <button type="submit" disabled={step === 1 && !scheduleReady} className={primaryButton}>
            Continue
          </button>
        ) : (
          <a href="/docs/display-toml" className="text-sm text-muted hover:text-fg">
            Read about display.toml
          </a>
        )
      }
    >
      {step === 0 && (
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

      {step === 1 && (
        <div className="flex flex-col gap-8">
          <ScheduleEditor value={settings.schedule} onChange={(schedule) => update({ schedule })} />
          {settings.schedule.mode !== "always" && (
            <div className="flex flex-col gap-3">
              <Switch
                label="Turn the TV off and on over HDMI-CEC"
                checked={settings.tvPower}
                onChange={(tvPower) => update({ tvPower })}
              />
              <p className="text-[13px] leading-5 text-faint">
                Power the Pi from its own adapter, not the TV's USB port. If the TV powers the Pi, turning the TV off shuts the Pi down and it can't turn the TV back on.
              </p>
            </div>
          )}
        </div>
      )}

      {step === 2 && (
        <>
          <div className="flex flex-col gap-3">
            <Output
              label={<span className="font-mono">display.toml</span>}
              action={
                <span className="flex items-center gap-2">
                  <CopyButton text={toml} secondary />
                  <DownloadButton name="display.toml" text={toml} />
                </span>
              }
            >
              <pre className="font-mono text-sm leading-6">{toml}</pre>
            </Output>
            <p className="text-[13px] leading-5 text-faint">
              Save it as display.toml next to your images. Changes show up within a slide, plus up to 30 seconds of sync for SMB and Drive.
            </p>
          </div>

          <div className="flex flex-col gap-4 border-t border-line pt-8">
            <Switch
              label={
                <span className="flex flex-col gap-1">
                  <span className="font-semibold">Apply to this Pi only</span>
                  <span className="text-sm text-muted">For one screen that plays differently, or a folder you can't add files to.</span>
                </span>
              }
              checked={onePi}
              onChange={setOnePi}
            />
            {onePi && (
              <>
                <Output label="Terminal on the Pi" action={<CopyButton text={configCommand(settings)} />}>
                  <p className="font-mono text-sm leading-6 break-all">{configCommand(settings)}</p>
                </Output>
                <p className="text-[13px] leading-5 text-faint">
                  Saves these settings on the Pi and restarts the slideshow. A display.toml in the folder still wins for any key it sets.
                </p>
              </>
            )}
          </div>
        </>
      )}
    </Wizard>
  );
}

function DownloadButton({ name, text }: { name: string; text: string }) {
  return (
    <button
      type="button"
      className={`${smallButton} flex items-center gap-1.5 bg-fg text-bg hover:bg-muted`}
      onClick={() => {
        const url = URL.createObjectURL(new Blob([text], { type: "application/toml" }));
        const link = Object.assign(document.createElement("a"), { href: url, download: name });
        link.click();
        URL.revokeObjectURL(url);
      }}
    >
      <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
        <path d="M6 1.5 V8 M3 5.5 L6 8.5 L9 5.5 M2 10.5 H10" stroke="currentColor" strokeWidth="1.4" />
      </svg>
      Download
    </button>
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
