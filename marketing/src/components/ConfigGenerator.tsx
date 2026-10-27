import { useEffect, useId, useState, type ReactNode } from "react";
import {
  defaultSettings,
  isValidSeconds,
  maxSeconds,
  setupCommand,
  type Fit,
  type Settings,
  type SourceKind,
} from "../lib/setup";

const input =
  "h-11 w-full rounded-md border border-edge bg-transparent px-3.5 text-[15px] outline-none placeholder:text-faint focus:border-fg aria-invalid:border-red-500";

// Form on the left, the generated setup command on the right. Nothing leaves
// the browser: the command is computed from local state on render.
export default function ConfigGenerator() {
  const [settings, setSettings] = useState<Settings>(defaultSettings);
  const update = (patch: Partial<Settings>) => setSettings((s) => ({ ...s, ...patch }));

  const command = setupCommand(settings);
  // Only flag the address once something is typed, so an empty field isn't red.
  const sourceInvalid = settings.source.trim() !== "" && !command;

  return (
    <div className="grid items-start gap-16 lg:grid-cols-[480px_1fr]">
      <form className="flex flex-col gap-8" onSubmit={(e) => e.preventDefault()}>
        <Section title="Where is your media?">
          <SourcePicker value={settings.kind} onChange={(kind) => update({ kind, source: "" })} />
          {settings.kind === "smb" && (
            <>
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
              <p className="-mt-2 text-[13px] text-faint">Leave both empty for a guest share.</p>
            </>
          )}
          {settings.kind === "drive" && (
            <Field label="Folder link" hint="Share the folder as Anyone with the link, then paste the link here.">
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
          )}
        </Section>

        {settings.kind && (
          <>
            <Section title="Playback">
              <div className="flex gap-4">
                <div className="w-36 shrink-0">
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
                  <FitToggle labelledBy="fit-label" value={settings.fit} onChange={(fit) => update({ fit })} />
                </div>
              </div>
              <Switch label="Mute video audio" checked={settings.mute} onChange={(mute) => update({ mute })} />
              <Switch label="Shuffle the order" checked={settings.shuffle} onChange={(shuffle) => update({ shuffle })} />
            </Section>

            <Section title="Longer slides" description="Optional. Give specific files more time.">
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
                        <button
                          type="button"
                          aria-label={`Remove ${override.name || "file"}`}
                          className="flex size-8 shrink-0 items-center justify-center rounded text-faint hover:text-fg"
                          onClick={() => update({ overrides: settings.overrides.filter((_, j) => j !== i) })}
                        >
                          <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">
                            <path d="M2 2 L10 10 M10 2 L2 10" stroke="currentColor" strokeWidth="1.5" />
                          </svg>
                        </button>
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
            </Section>

            <Section title="Schedule" description="Optional. Turn the screen off outside these hours.">
              <Switch
                label="Only show during set hours"
                checked={settings.schedule !== null}
                onChange={(on) => update({ schedule: on ? { on: "07:00", off: "22:00" } : null })}
              />
              {settings.schedule && (
                <div className="flex gap-4">
                  {(["on", "off"] as const).map((key) => (
                    <Field key={key} label={key === "on" ? "Screen on" : "Screen off"}>
                      {(id) => (
                        <input
                          id={id}
                          type="time"
                          className={`${input} [color-scheme:dark]`}
                          value={settings.schedule?.[key] ?? ""}
                          onChange={(e) =>
                            settings.schedule && update({ schedule: { ...settings.schedule, [key]: e.target.value } })
                          }
                        />
                      )}
                    </Field>
                  ))}
                </div>
              )}
            </Section>
          </>
        )}
      </form>

      <div className="flex flex-col gap-10 lg:sticky lg:top-8">
        <Step title="Run this on the Pi" description="Saves every setting above, tests the connection, and starts playback.">
          <Output label="Terminal" action={command ? <CopyButton text={command} /> : null}>
            {command ? (
              <p className="font-mono text-sm leading-[22px] break-all">{command}</p>
            ) : (
              <p className="text-sm text-faint">
                {!settings.kind
                  ? "Pick where your media lives to get your command."
                  : settings.kind === "smb"
                    ? "Enter the share address to get your command."
                    : "Paste the folder link to get your command."}
              </p>
            )}
          </Output>
          {command && settings.kind === "smb" && settings.password && (
            <p className="text-[13px] text-faint">The command includes your password, encoded but not encrypted. Share it only with people allowed to know it.</p>
          )}
        </Step>
      </div>
    </div>
  );
}

function Section({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
  return (
    <fieldset className="flex flex-col gap-5 border-t border-line pt-8 first:border-t-0 first:pt-0">
      <legend className="contents">
        <span className="text-[17px] font-semibold">{title}</span>
      </legend>
      {description && <p className="-mt-4 text-sm text-muted">{description}</p>}
      {children}
    </fieldset>
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

function SourcePicker({ value, onChange }: { value: SourceKind | null; onChange: (kind: SourceKind) => void }) {
  const options: { kind: SourceKind; label: string; detail: string }[] = [
    { kind: "smb", label: "SMB share", detail: "A NAS or shared folder on your network" },
    { kind: "drive", label: "Google Drive", detail: "A folder shared by link" },
  ];
  return (
    <div role="radiogroup" aria-label="Media source" className="grid grid-cols-2 gap-3">
      {options.map((option) => (
        <button
          key={option.kind}
          type="button"
          role="radio"
          aria-checked={value === option.kind}
          className="flex flex-col gap-1 rounded-md border border-edge p-4 text-left hover:border-muted aria-checked:border-fg"
          onClick={() => value !== option.kind && onChange(option.kind)}
        >
          <span className="text-[15px] font-medium">{option.label}</span>
          <span className="text-[13px] text-faint">{option.detail}</span>
        </button>
      ))}
    </div>
  );
}

function FitToggle({ labelledBy, value, onChange }: { labelledBy: string; value: Fit; onChange: (fit: Fit) => void }) {
  const options: { fit: Fit; label: string }[] = [
    { fit: "contain", label: "Contain" },
    { fit: "cover", label: "Cover" },
  ];
  return (
    <div role="radiogroup" aria-labelledby={labelledBy} className="flex h-11 gap-1 rounded-md border border-edge p-1">
      {options.map((option) => (
        <button
          key={option.fit}
          type="button"
          role="radio"
          aria-checked={value === option.fit}
          className="flex-1 rounded text-sm text-muted aria-checked:bg-line aria-checked:text-fg"
          onClick={() => onChange(option.fit)}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

function Switch({ label, checked, onChange }: { label: string; checked: boolean; onChange: (checked: boolean) => void }) {
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

function Step({ title, description, children }: { title: string; description: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h2 className="text-[17px] font-semibold">{title}</h2>
        <p className="text-sm text-muted">{description}</p>
      </div>
      {children}
    </section>
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
