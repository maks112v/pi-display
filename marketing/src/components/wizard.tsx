import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { isValidSeconds, maxSeconds } from "../lib/setup";

// Building blocks shared by the install and config wizards.

export const input =
  "h-11 w-full rounded-md border border-edge bg-transparent px-3.5 text-[15px] outline-none placeholder:text-faint focus:border-fg aria-invalid:border-red-500";

export const primaryButton =
  "flex h-10 items-center rounded-md bg-fg px-[18px] text-sm font-medium text-bg hover:bg-muted disabled:bg-edge disabled:text-faint";

export interface WizardStep {
  label: string;
  title: string;
  description: string;
  // Shown under the label once the step is done.
  summary?: string | null;
}

// Step navigation shared by both wizards: which step is open, the furthest
// one reached (finished steps stay clickable), and focus on the new heading.
export function useWizard() {
  const [step, setStep] = useState(0);
  const [reached, setReached] = useState(0);
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
  return { step, reached, go, heading };
}

// The step list on the left and the open step's heading, form, and footer.
// canOpen(i) says whether step i can be opened; a step counts as done once
// the one after it can open.
export function Wizard({
  steps,
  step,
  reached,
  go,
  heading,
  canOpen,
  canLeave,
  children,
  footer,
}: ReturnType<typeof useWizard> & {
  steps: WizardStep[];
  canOpen: (i: number) => boolean;
  canLeave: boolean;
  children: ReactNode;
  footer: ReactNode;
}) {
  const current = steps[step];
  return (
    <div className="grid items-start gap-12 lg:grid-cols-[224px_minmax(0,720px)] lg:gap-24">
      <ol className="hidden border-l border-line lg:flex lg:flex-col">
        {steps.map(({ label, summary }, i) => {
          const open = i === step;
          const done = i < reached && !open && canOpen(i + 1);
          return (
            <li key={label}>
              <button
                type="button"
                disabled={open || i > reached || !canOpen(i)}
                aria-current={open ? "step" : undefined}
                onClick={() => go(i)}
                className="-ml-px flex w-full items-center gap-3.5 border-l-2 border-transparent py-2.5 pl-5 text-left text-[15px] text-muted enabled:hover:text-fg aria-[current=step]:border-accent aria-[current=step]:font-medium aria-[current=step]:text-fg"
              >
                <span className="flex w-4 shrink-0 justify-center font-mono text-[13px] text-faint">
                  {open ? (
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
                  {done && summary && <span className="truncate font-mono text-xs font-normal text-faint">{summary}</span>}
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
            {current?.title}
          </h2>
          <p className="text-[15px] leading-6 text-muted">{current?.description}</p>
        </div>
        {children}
        <div className="flex items-center justify-between gap-4">
          <button
            type="button"
            className="text-sm text-muted hover:text-fg disabled:invisible"
            disabled={step === 0}
            onClick={() => go(step - 1)}
          >
            Back
          </button>
          <div className="flex items-center gap-5">{footer}</div>
        </div>
      </form>
    </div>
  );
}

// A two-column table: a command (or step number) beside a plain explanation.
// Plain-string rows are numbered instead, with a narrow, muted label column.
export function Explain({ title, rows }: { title: string; rows: string[] | [string, string][] }) {
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
export function Field({ label, hint, children }: { label: string; hint?: string; children: (id: string) => ReactNode }) {
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

export function SecondsInput({
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

// A row of mutually exclusive options, as [value, label] pairs.
export function Segmented<T extends string>({
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

export function Switch({ label, checked, onChange }: { label: ReactNode; checked: boolean; onChange: (checked: boolean) => void }) {
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

export function Output({ label, action, children }: { label: ReactNode; action: ReactNode; children: ReactNode }) {
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

export const smallButton = "rounded-[5px] px-3 py-1.5 text-[13px] font-medium";

// Saves text as a file named name, built in the browser.
export function download(name: string, text: string) {
  const url = URL.createObjectURL(new Blob([text], { type: "text/plain" }));
  Object.assign(document.createElement("a"), { href: url, download: name }).click();
  URL.revokeObjectURL(url);
}

export function CopyButton({ text, secondary = false }: { text: string; secondary?: boolean }) {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  return (
    <button
      type="button"
      className={`${smallButton} ${secondary ? "border border-edge text-fg hover:border-muted" : "bg-fg text-bg hover:bg-muted"}`}
      onClick={() => navigator.clipboard.writeText(text).then(() => setCopied(true))}
    >
      {copied ? "Copied" : "Copy"}
    </button>
  );
}
