"use client";

import clsx from "clsx";
import { Activity, CircleDot, Cpu, Layers, Server } from "lucide-react";
import { useClusterUtilization } from "@/lib/hooks";
import { formatPercent } from "@/lib/format";
import { ThemeToggle } from "./ThemeToggle";

interface HealthLevel {
  label: string;
  dot: string;
  text: string;
}

function healthFromNodes(ready: number, total: number): HealthLevel {
  if (total === 0) {
    return {
      label: "No nodes",
      dot: "bg-slate-400",
      text: "text-slate-500 dark:text-slate-400",
    };
  }
  const ratio = ready / total;
  if (ratio >= 0.99) {
    return {
      label: "Healthy",
      dot: "bg-emerald-500",
      text: "text-emerald-600 dark:text-emerald-400",
    };
  }
  if (ratio >= 0.6) {
    return {
      label: "Degraded",
      dot: "bg-amber-500",
      text: "text-amber-600 dark:text-amber-400",
    };
  }
  return {
    label: "Critical",
    dot: "bg-rose-500",
    text: "text-rose-600 dark:text-rose-400",
  };
}

function Metric({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Cpu;
  label: string;
  value: string;
}) {
  return (
    <div className="hidden items-center gap-2 md:flex">
      <Icon className="h-4 w-4 text-slate-400 dark:text-slate-500" aria-hidden />
      <span className="text-xs text-slate-500 dark:text-slate-400">{label}</span>
      <span className="text-xs font-semibold tabular-nums text-slate-800 dark:text-slate-100">
        {value}
      </span>
    </div>
  );
}

export function TopBar() {
  const { data, error, isLoading } = useClusterUtilization();

  const connected = !error;
  const health = data
    ? healthFromNodes(data.nodes_ready, data.nodes_total)
    : null;

  return (
    <header className="sticky top-0 z-20 flex items-center justify-between gap-4 border-b border-slate-200 bg-white/85 px-4 py-3 backdrop-blur md:px-6 dark:border-slate-800 dark:bg-slate-950/85">
      <div className="flex items-center gap-3">
        {health ? (
          <span
            className={clsx(
              "inline-flex items-center gap-2 rounded-full border border-slate-200 px-3 py-1 dark:border-slate-800",
            )}
          >
            <span
              className={clsx(
                "h-2 w-2 rounded-full",
                health.dot,
                connected && "animate-pulse",
              )}
              aria-hidden
            />
            <span className={clsx("text-xs font-semibold", health.text)}>
              {connected ? health.label : "Disconnected"}
            </span>
          </span>
        ) : (
          <span className="inline-flex items-center gap-2 rounded-full border border-slate-200 px-3 py-1 dark:border-slate-800">
            <CircleDot className="h-3.5 w-3.5 text-slate-400" aria-hidden />
            <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
              {error ? "Disconnected" : isLoading ? "Connecting…" : "—"}
            </span>
          </span>
        )}
      </div>

      <div className="flex items-center gap-5">
        {data ? (
          <>
            <Metric
              icon={Server}
              label="Nodes"
              value={`${data.nodes_ready}/${data.nodes_total}`}
            />
            <Metric
              icon={Cpu}
              label="GPU"
              value={formatPercent(data.gpu_utilization)}
            />
            <Metric
              icon={Activity}
              label="Running"
              value={String(data.jobs_running)}
            />
            <Metric
              icon={Layers}
              label="Queued"
              value={String(data.jobs_queued)}
            />
          </>
        ) : null}
        <ThemeToggle />
      </div>
    </header>
  );
}
