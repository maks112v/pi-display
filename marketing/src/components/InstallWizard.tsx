import { useState, type ReactNode } from "react";
import { defaultSource, installCommand, normalizeSource, setupCommand, type SourceKind, type SourceSettings } from "../lib/setup";
import { CopyButton, Explain, Field, Output, Wizard, input, primaryButton, useWizard, type WizardStep } from "./wizard";

// The install page: download pi-display, pick the media source, run setup.
// Playback settings live on the config page, which the last step links to.
// Nothing leaves the browser: the command is computed from local state.
export default function InstallWizard() {
  const [settings, setSettings] = useState<SourceSettings>(defaultSource);
  const update = (patch: Partial<SourceSettings>) => setSettings((s) => ({ ...s, ...patch }));
  const wizard = useWizard();
  const { step, go } = wizard;

  const source = normalizeSource(settings.kind, settings.source);
  const command = setupCommand(settings);
  // Only flag the address once something is typed, so an empty field isn't red.
  const sourceInvalid = settings.source.trim() !== "" && !source;

  const steps: WizardStep[] = [
    {
      label: "Install",
      title: "Install pi-display",
      description: "SSH into the Pi and run these two commands. New Pi? Flash Raspberry Pi OS Lite with SSH turned on first.",
    },
    {
      label: "Media source",
      title: "Where is your media?",
      description: "The Pi copies every image and video from this folder and keeps it in sync.",
      summary: settings.kind === "drive" ? "Google Drive folder" : (source?.replace(/^smb:\/\//, "") ?? null),
    },
    {
      label: "Run setup",
      title: "Run setup",
      description:
        "This connects the Pi to your folder and starts the slideshow with default playback. Set timing, schedules, and more in the config generator next.",
    },
  ];

  return (
    <Wizard
      {...wizard}
      steps={steps}
      canOpen={(i) => i <= 1 || source !== null}
      canLeave={step !== 1 || source !== null}
      footer={
        step === 0 ? (
          <>
            <button type="button" className="text-sm text-muted hover:text-fg" onClick={() => go(1)}>
              Already installed? Skip
            </button>
            <button type="submit" className={primaryButton}>
              Continue
            </button>
          </>
        ) : step === 1 ? (
          <button type="submit" disabled={source === null} className={primaryButton}>
            Continue
          </button>
        ) : (
          <>
            <a href="/docs" className="text-sm text-muted hover:text-fg">
              Done for now
            </a>
            <a href="/config" className={primaryButton}>
              Continue to config
            </a>
          </>
        )
      }
    >
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
                Leave both empty for a guest share. To have each screen write its log to the share, the login needs write access.
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
              <p className="text-[13px] leading-5 text-faint">Only files at the top of the folder play. Native Google Docs are skipped.</p>
            </div>
          )}
        </>
      )}

      {step === 2 && command && (
        <>
          <div className="flex flex-col gap-3">
            <Output label="Terminal on the Pi" action={<CopyButton text={command} />}>
              <p className="font-mono text-sm leading-6 break-all">{command}</p>
            </Output>
            {settings.kind === "smb" && settings.username && settings.password && (
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
        </>
      )}
    </Wizard>
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
