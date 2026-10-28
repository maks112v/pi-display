import { useState } from "react";
import { CopyButton, Segmented, download, input, primaryButton } from "./wizard";

// The screen command page: pick on, off, or back to schedule and a duration,
// then download a one-line screen.txt for the media folder. screenfile.go
// reads it as `on|off [duration]`, with `on 0` cancelling.

type Action = "off" | "on" | "schedule";

const presets = ["15m", "30m", "1h", "2h", "4h"];

// Hours and minutes only, a subset of Go's duration format the Pi parses.
const isValidDuration = (value: string) => /^(?=\d)(\d+h)?(\d+m)?$/.test(value) && /[1-9]/.test(value);

export default function ScreenCommand() {
  const [action, setAction] = useState<Action>("off");
  const [duration, setDuration] = useState("30m");
  const custom = !presets.includes(duration);
  const valid = action === "schedule" || isValidDuration(duration);
  const [verb, arg] = action === "schedule" ? ["on", "0"] : [action, duration];
  const line = `${verb} ${arg}`;

  return (
    <div className="flex flex-col items-start gap-14 lg:flex-row lg:gap-24">
      <div className="flex w-full flex-col gap-10 lg:w-[600px] lg:shrink-0">
        <div className="flex flex-col gap-2">
          <span id="action-label" className="text-sm text-muted">
            Screens
          </span>
          <Segmented
            labelledBy="action-label"
            options={[
              ["off", "Off"],
              ["on", "On"],
              ["schedule", "Back to schedule"],
            ]}
            value={action}
            onChange={setAction}
          />
        </div>

        {action !== "schedule" && (
          <div className="flex flex-col gap-2">
            <span id="duration-label" className="text-sm text-muted">
              For
            </span>
            <div role="radiogroup" aria-labelledby="duration-label" className="flex flex-wrap gap-2 font-mono text-sm">
              {presets.map((preset) => (
                <button
                  key={preset}
                  type="button"
                  role="radio"
                  aria-checked={duration === preset}
                  className="h-11 w-[72px] rounded-md border border-edge text-muted hover:border-muted aria-checked:border-fg aria-checked:bg-line aria-checked:text-fg"
                  onClick={() => setDuration(preset)}
                >
                  {preset}
                </button>
              ))}
              <input
                aria-label="Other duration"
                className={`${input} min-w-32 flex-1 font-mono text-sm ${custom && duration ? "border-fg" : ""}`}
                placeholder="1h30m"
                value={custom ? duration : ""}
                aria-invalid={custom && duration !== "" && !valid}
                onChange={(e) => setDuration(e.target.value.trim().toLowerCase())}
                spellCheck={false}
              />
            </div>
            <p className="text-[13px] leading-5 text-faint">Any duration in hours and minutes, like 45m or 3h30m.</p>
          </div>
        )}

        <div className="flex flex-col gap-5 border-t border-line pt-8">
          <h2 className="text-[15px] font-semibold">What happens</h2>
          {action === "schedule" ? (
            <p className="text-sm leading-[22px] text-muted">
              Cancels any on or off command, from screen.txt or from <code className="font-mono text-fg">pi-display on</code> on a
              Pi, and every screen goes back to its schedule.
            </p>
          ) : (
            <>
              <Timeline action={action} duration={valid ? duration : "…"} />
              <ul className="border-t border-line">
                {[
                  ["Same end time", "The time counts from when the file was saved, so every screen switches back together."],
                  ["Late Pis catch up", "A Pi that was offline gets only the time that's left. One that comes back after it ends skips it."],
                  ["Runs once", "Each save runs once. To send it again, save the file again, even with the same words."],
                ].map(([title, text]) => (
                  <li key={title} className="flex flex-col gap-1 border-b border-line py-3.5 text-sm leading-[22px] sm:flex-row sm:gap-6">
                    <span className="shrink-0 font-medium sm:w-[140px]">{title}</span>
                    <span className="text-muted">{text}</span>
                  </li>
                ))}
              </ul>
            </>
          )}
        </div>
      </div>

      <div className="flex w-full min-w-0 flex-1 flex-col gap-5">
        <div className="rounded-lg border border-edge">
          <div className="flex h-12 items-center justify-between border-b border-line px-4 text-[13px] text-muted">
            <span className="font-mono">screen.txt</span>
            <span>1 line</span>
          </div>
          <div className="rounded-b-lg bg-screen px-6 py-10">
            <p className="font-mono text-[40px] leading-12 font-medium tracking-[-0.02em]">
              {verb} <span className={valid ? "text-accent" : "text-faint"}>{valid ? arg : "…"}</span>
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2.5">
          <button type="button" disabled={!valid} className={`${primaryButton} gap-2`} onClick={() => download("screen.txt", `${line}\n`)}>
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
              <path d="M7 1.5 V9.5 M3.5 6 L7 9.5 L10.5 6 M2 12.5 H12" stroke="currentColor" strokeWidth="1.5" />
            </svg>
            Download screen.txt
          </button>
          {valid && <CopyButton text={line} secondary />}
        </div>

        <div className="flex flex-col gap-3.5 pt-4">
          <h2 className="text-[15px] font-semibold">Drop it next to display.toml</h2>
          <pre className="rounded-lg border border-line p-4 font-mono text-[13px] leading-6 text-muted">
            <span className="text-faint">Lobby screens/</span>
            {"\n  display.toml\n  "}
            <span className="text-fg">screen.txt</span> <span className="text-accent">← new</span>
            {"\n  welcome.jpg\n  menu.mp4"}
          </pre>
          <p className="text-sm leading-[22px] text-muted">
            Every screen on this folder picks it up within about 30 seconds. On Google Drive, upload the .txt file itself. A Google
            Doc won't work.{" "}
            <a href="/docs/screen-txt" className="text-fg underline decoration-edge underline-offset-4 hover:decoration-fg">
              More on screen.txt
            </a>
          </p>
        </div>
      </div>
    </div>
  );
}

// A bar of before, during, and after the command: white where the screen is
// on, dark where it's off, gray where the schedule decides.
function Timeline({ action, duration }: { action: "on" | "off"; duration: string }) {
  const edge = action === "off" ? "bg-fg" : "bg-edge";
  const middle = action === "off" ? "bg-line" : "bg-fg";
  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex h-2">
        <div className={`w-[6%] rounded-l ${edge}`} />
        <div className={`flex-1 ${middle}`} />
        <div className={`w-[22%] rounded-r ${edge}`} />
      </div>
      <div className="flex justify-between font-mono text-[13px] text-muted">
        <span>You save</span>
        <span>
          {action === "off" ? "Off" : "On"} for {duration}
        </span>
        <span className="text-fg">Back to schedule</span>
      </div>
    </div>
  );
}
