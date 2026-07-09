import clsx from "clsx";

type Tone = "brand" | "emerald" | "amber" | "rose" | "sky" | "violet";

const toneClasses: Record<Tone, string> = {
  brand: "bg-brand-500",
  emerald: "bg-emerald-500",
  amber: "bg-amber-500",
  rose: "bg-rose-500",
  sky: "bg-sky-500",
  violet: "bg-violet-500",
};

interface ProgressBarProps {
  /** Value between 0 and 1. */
  value: number;
  tone?: Tone;
  /** Automatically shift tone to amber/rose as utilization climbs. */
  autoTone?: boolean;
  className?: string;
  height?: "sm" | "md";
}

function resolveTone(value: number, base: Tone, autoTone?: boolean): Tone {
  if (!autoTone) return base;
  if (value >= 0.9) return "rose";
  if (value >= 0.75) return "amber";
  return base;
}

export function ProgressBar({
  value,
  tone = "brand",
  autoTone = false,
  className,
  height = "md",
}: ProgressBarProps) {
  const clamped = Math.max(0, Math.min(1, Number.isFinite(value) ? value : 0));
  const resolved = resolveTone(clamped, tone, autoTone);

  return (
    <div
      className={clsx(
        "w-full overflow-hidden rounded-full bg-slate-200/80 dark:bg-slate-700/60",
        height === "sm" ? "h-1.5" : "h-2.5",
        className,
      )}
      role="progressbar"
      aria-valuenow={Math.round(clamped * 100)}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <div
        className={clsx(
          "h-full rounded-full transition-[width] duration-500 ease-out",
          toneClasses[resolved],
        )}
        style={{ width: `${clamped * 100}%` }}
      />
    </div>
  );
}
