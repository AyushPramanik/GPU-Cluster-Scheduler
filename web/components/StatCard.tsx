import clsx from "clsx";
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

type Accent = "brand" | "emerald" | "amber" | "rose" | "sky" | "violet" | "slate";

const accentClasses: Record<Accent, string> = {
  brand: "text-brand-500 bg-brand-500/10",
  emerald: "text-emerald-500 bg-emerald-500/10",
  amber: "text-amber-500 bg-amber-500/10",
  rose: "text-rose-500 bg-rose-500/10",
  sky: "text-sky-500 bg-sky-500/10",
  violet: "text-violet-500 bg-violet-500/10",
  slate: "text-slate-500 bg-slate-500/10",
};

interface StatCardProps {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  icon?: LucideIcon;
  accent?: Accent;
  children?: ReactNode;
  className?: string;
}

export function StatCard({
  label,
  value,
  hint,
  icon: Icon,
  accent = "brand",
  children,
  className,
}: StatCardProps) {
  return (
    <div
      className={clsx(
        "group rounded-xl border border-slate-200 bg-white p-5 shadow-sm transition-colors",
        "dark:border-slate-800 dark:bg-slate-900",
        className,
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-medium uppercase tracking-wide text-slate-500 dark:text-slate-400">
            {label}
          </p>
          <p className="mt-2 text-2xl font-semibold tabular-nums text-slate-900 dark:text-slate-50">
            {value}
          </p>
        </div>
        {Icon ? (
          <span
            className={clsx(
              "inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg",
              accentClasses[accent],
            )}
          >
            <Icon className="h-5 w-5" aria-hidden />
          </span>
        ) : null}
      </div>

      {children ? <div className="mt-4">{children}</div> : null}

      {hint ? (
        <p className="mt-3 text-xs text-slate-500 dark:text-slate-400">{hint}</p>
      ) : null}
    </div>
  );
}
