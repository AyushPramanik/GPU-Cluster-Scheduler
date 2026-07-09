import clsx from "clsx";
import type { JobStatus, NodeStatus } from "@/lib/types";

const jobStatusStyles: Record<JobStatus, string> = {
  queued:
    "bg-slate-100 text-slate-600 ring-slate-500/20 dark:bg-slate-700/40 dark:text-slate-300",
  scheduled:
    "bg-sky-100 text-sky-700 ring-sky-500/20 dark:bg-sky-500/15 dark:text-sky-300",
  running:
    "bg-brand-100 text-brand-700 ring-brand-500/20 dark:bg-brand-500/15 dark:text-brand-300",
  completed:
    "bg-emerald-100 text-emerald-700 ring-emerald-500/20 dark:bg-emerald-500/15 dark:text-emerald-300",
  failed:
    "bg-rose-100 text-rose-700 ring-rose-500/20 dark:bg-rose-500/15 dark:text-rose-300",
  cancelled:
    "bg-slate-100 text-slate-500 ring-slate-500/20 dark:bg-slate-700/40 dark:text-slate-400",
  preempted:
    "bg-amber-100 text-amber-700 ring-amber-500/20 dark:bg-amber-500/15 dark:text-amber-300",
};

const nodeStatusStyles: Record<NodeStatus, string> = {
  ready:
    "bg-emerald-100 text-emerald-700 ring-emerald-500/20 dark:bg-emerald-500/15 dark:text-emerald-300",
  draining:
    "bg-amber-100 text-amber-700 ring-amber-500/20 dark:bg-amber-500/15 dark:text-amber-300",
  cordoned:
    "bg-sky-100 text-sky-700 ring-sky-500/20 dark:bg-sky-500/15 dark:text-sky-300",
  down: "bg-rose-100 text-rose-700 ring-rose-500/20 dark:bg-rose-500/15 dark:text-rose-300",
};

const pulseStatuses = new Set<JobStatus | NodeStatus>(["running", "ready"]);

interface StatusBadgeProps {
  status: JobStatus | NodeStatus;
  kind?: "job" | "node";
  className?: string;
}

export function StatusBadge({ status, kind = "job", className }: StatusBadgeProps) {
  const styles =
    kind === "node"
      ? nodeStatusStyles[status as NodeStatus]
      : jobStatusStyles[status as JobStatus];

  return (
    <span
      className={clsx(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ring-1 ring-inset",
        styles,
        className,
      )}
    >
      <span
        className={clsx(
          "h-1.5 w-1.5 rounded-full bg-current",
          pulseStatuses.has(status) && "animate-pulse",
        )}
        aria-hidden
      />
      {status}
    </span>
  );
}
