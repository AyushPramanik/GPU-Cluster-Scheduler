import clsx from "clsx";
import { AlertTriangle, Inbox, Loader2, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

export function LoadingState({
  label = "Loading…",
  className,
}: {
  label?: string;
  className?: string;
}) {
  return (
    <div
      className={clsx(
        "flex flex-col items-center justify-center gap-3 py-16 text-slate-500 dark:text-slate-400",
        className,
      )}
    >
      <Loader2 className="h-6 w-6 animate-spin text-brand-500" aria-hidden />
      <span className="text-sm">{label}</span>
    </div>
  );
}

export function ErrorState({
  error,
  onRetry,
  className,
}: {
  error: unknown;
  onRetry?: () => void;
  className?: string;
}) {
  const message =
    error instanceof Error ? error.message : "Something went wrong.";

  return (
    <div
      className={clsx(
        "flex flex-col items-center justify-center gap-3 rounded-xl border border-rose-200 bg-rose-50 py-12 text-center dark:border-rose-500/30 dark:bg-rose-500/10",
        className,
      )}
      role="alert"
    >
      <AlertTriangle className="h-6 w-6 text-rose-500" aria-hidden />
      <div>
        <p className="text-sm font-medium text-rose-700 dark:text-rose-300">
          Failed to load data
        </p>
        <p className="mt-1 max-w-sm text-xs text-rose-600/80 dark:text-rose-300/70">
          {message}
        </p>
      </div>
      {onRetry ? (
        <button
          type="button"
          onClick={onRetry}
          className="mt-1 rounded-lg border border-rose-300 bg-white px-3 py-1.5 text-xs font-medium text-rose-700 transition-colors hover:bg-rose-100 dark:border-rose-500/40 dark:bg-transparent dark:text-rose-300 dark:hover:bg-rose-500/10"
        >
          Retry
        </button>
      ) : null}
    </div>
  );
}

export function EmptyState({
  title,
  description,
  icon: Icon = Inbox,
  action,
  className,
}: {
  title: string;
  description?: string;
  icon?: LucideIcon;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={clsx(
        "flex flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-slate-300 py-14 text-center dark:border-slate-700",
        className,
      )}
    >
      <Icon className="h-7 w-7 text-slate-400 dark:text-slate-500" aria-hidden />
      <p className="text-sm font-medium text-slate-700 dark:text-slate-200">
        {title}
      </p>
      {description ? (
        <p className="max-w-sm text-xs text-slate-500 dark:text-slate-400">
          {description}
        </p>
      ) : null}
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  );
}
