"use client";

import {
  Activity,
  Cpu,
  Layers,
  MemoryStick,
  Puzzle,
  Server,
  Zap,
} from "lucide-react";
import { Card, CardHeader } from "@/components/Card";
import { PageHeader } from "@/components/PageHeader";
import { ProgressBar } from "@/components/ProgressBar";
import { StatCard } from "@/components/StatCard";
import { ErrorState, LoadingState } from "@/components/States";
import { useClusterUtilization } from "@/lib/hooks";
import { formatMemory, formatNumber, formatPercent } from "@/lib/format";
import type { ClusterUtilization } from "@/lib/types";

function UtilizationStat({
  label,
  icon,
  accent,
  used,
  total,
  ratio,
  unit,
  formatValue,
}: {
  label: string;
  icon: typeof Cpu;
  accent: "brand" | "sky" | "violet";
  used: number;
  total: number;
  ratio: number;
  unit?: string;
  formatValue?: (v: number) => string;
}) {
  const fmt = formatValue ?? formatNumber;
  return (
    <StatCard
      label={label}
      icon={icon}
      accent={accent}
      value={formatPercent(ratio, 1)}
      hint={
        <>
          {fmt(used)}
          {unit ? ` ${unit}` : ""} of {fmt(total)}
          {unit ? ` ${unit}` : ""} allocated
        </>
      }
    >
      <ProgressBar value={ratio} tone={accent} autoTone />
    </StatCard>
  );
}

function SummaryRow({
  label,
  used,
  total,
  ratio,
  tone,
  formatValue,
}: {
  label: string;
  used: number;
  total: number;
  ratio: number;
  tone: "brand" | "sky" | "violet";
  formatValue: (v: number) => string;
}) {
  return (
    <div className="space-y-2">
      <div className="flex items-baseline justify-between">
        <span className="text-sm font-medium text-slate-700 dark:text-slate-200">
          {label}
        </span>
        <span className="text-xs tabular-nums text-slate-500 dark:text-slate-400">
          {formatValue(used)} / {formatValue(total)} ·{" "}
          <span className="font-semibold text-slate-700 dark:text-slate-200">
            {formatPercent(ratio, 1)}
          </span>
        </span>
      </div>
      <ProgressBar value={ratio} tone={tone} autoTone />
    </div>
  );
}

function Overview({ data }: { data: ClusterUtilization }) {
  const nodesRatio = data.nodes_total > 0 ? data.nodes_ready / data.nodes_total : 0;

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <UtilizationStat
          label="GPU Utilization"
          icon={Cpu}
          accent="brand"
          used={data.used_gpus}
          total={data.total_gpus}
          ratio={data.gpu_utilization}
        />
        <UtilizationStat
          label="CPU Utilization"
          icon={Zap}
          accent="sky"
          used={data.used_cpus}
          total={data.total_cpus}
          ratio={data.cpu_utilization}
        />
        <UtilizationStat
          label="Memory Utilization"
          icon={MemoryStick}
          accent="violet"
          used={data.used_memory_gb}
          total={data.total_memory_gb}
          ratio={data.memory_utilization}
          formatValue={formatMemory}
        />
      </div>

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard
          label="Nodes Ready"
          icon={Server}
          accent="emerald"
          value={`${data.nodes_ready} / ${data.nodes_total}`}
          hint={`${formatPercent(nodesRatio)} of fleet online`}
        />
        <StatCard
          label="Jobs Running"
          icon={Activity}
          accent="brand"
          value={formatNumber(data.jobs_running)}
          hint="Actively executing"
        />
        <StatCard
          label="Jobs Queued"
          icon={Layers}
          accent="amber"
          value={formatNumber(data.jobs_queued)}
          hint="Awaiting placement"
        />
        <StatCard
          label="Fragmentation"
          icon={Puzzle}
          accent={data.fragmentation >= 0.5 ? "rose" : "slate"}
          value={formatPercent(data.fragmentation, 1)}
          hint="Resource scatter across nodes"
        >
          <ProgressBar value={data.fragmentation} tone="amber" autoTone height="sm" />
        </StatCard>
      </div>

      <Card>
        <CardHeader
          title="Resource Utilization Summary"
          description="Aggregate allocation across the cluster, refreshed every 3 seconds."
        />
        <div className="grid grid-cols-1 gap-6 p-5 md:grid-cols-3">
          <SummaryRow
            label="GPUs"
            used={data.used_gpus}
            total={data.total_gpus}
            ratio={data.gpu_utilization}
            tone="brand"
            formatValue={formatNumber}
          />
          <SummaryRow
            label="CPUs"
            used={data.used_cpus}
            total={data.total_cpus}
            ratio={data.cpu_utilization}
            tone="sky"
            formatValue={formatNumber}
          />
          <SummaryRow
            label="Memory"
            used={data.used_memory_gb}
            total={data.total_memory_gb}
            ratio={data.memory_utilization}
            tone="violet"
            formatValue={formatMemory}
          />
        </div>
      </Card>
    </div>
  );
}

export default function OverviewPage() {
  const { data, error, isLoading, mutate } = useClusterUtilization();

  return (
    <>
      <PageHeader
        title="Cluster Overview"
        description="Real-time health and resource utilization across the GPU fleet."
      />
      {error && !data ? (
        <ErrorState error={error} onRetry={() => void mutate()} />
      ) : isLoading && !data ? (
        <LoadingState label="Loading cluster metrics…" />
      ) : data ? (
        <Overview data={data} />
      ) : null}
    </>
  );
}
