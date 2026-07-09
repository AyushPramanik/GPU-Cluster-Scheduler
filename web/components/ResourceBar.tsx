import clsx from "clsx";
import { ProgressBar } from "./ProgressBar";
import { formatPercent } from "@/lib/format";

interface ResourceBarProps {
  label: string;
  /** Amount currently in use. */
  used: number;
  /** Total capacity. */
  capacity: number;
  /** Optional unit suffix appended to the numeric readout. */
  unit?: string;
  tone?: "brand" | "emerald" | "sky" | "violet" | "amber";
  className?: string;
}

export function ResourceBar({
  label,
  used,
  capacity,
  unit,
  tone = "brand",
  className,
}: ResourceBarProps) {
  const ratio = capacity > 0 ? used / capacity : 0;
  const suffix = unit ? ` ${unit}` : "";

  return (
    <div className={clsx("space-y-1.5", className)}>
      <div className="flex items-baseline justify-between text-xs">
        <span className="font-medium text-slate-600 dark:text-slate-300">
          {label}
        </span>
        <span className="tabular-nums text-slate-500 dark:text-slate-400">
          {used}
          {suffix} / {capacity}
          {suffix}
          <span className="ml-1.5 font-medium text-slate-400 dark:text-slate-500">
            {formatPercent(ratio)}
          </span>
        </span>
      </div>
      <ProgressBar value={ratio} tone={tone} autoTone height="sm" />
    </div>
  );
}
