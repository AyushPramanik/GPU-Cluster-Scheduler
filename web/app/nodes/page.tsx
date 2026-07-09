"use client";

import { Server, Tag } from "lucide-react";
import { Card } from "@/components/Card";
import { PageHeader } from "@/components/PageHeader";
import { ResourceBar } from "@/components/ResourceBar";
import { StatusBadge } from "@/components/StatusBadge";
import { EmptyState, ErrorState, LoadingState } from "@/components/States";
import { formatMemory, timeAgo } from "@/lib/format";
import { useNodes } from "@/lib/hooks";
import type { ClusterNode } from "@/lib/types";

function NodeCard({ node }: { node: ClusterNode }) {
  const gpuUsed = node.gpu_capacity - node.gpu_available;
  const cpuUsed = node.cpu_capacity - node.cpu_available;
  const memUsed = node.memory_capacity - node.memory_available;
  const labels = node.labels ? Object.entries(node.labels) : [];

  return (
    <Card className="p-5">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <Server
              className="h-4 w-4 shrink-0 text-slate-400 dark:text-slate-500"
              aria-hidden
            />
            <h3 className="truncate text-sm font-semibold text-slate-900 dark:text-slate-100">
              {node.hostname || node.node_id}
            </h3>
          </div>
          <p className="mt-0.5 truncate font-mono text-[11px] text-slate-400">
            {node.node_id}
          </p>
        </div>
        <StatusBadge status={node.status} kind="node" />
      </div>

      <div className="mt-4 space-y-3">
        <ResourceBar
          label="GPU"
          used={gpuUsed}
          capacity={node.gpu_capacity}
          tone="brand"
        />
        <ResourceBar
          label="CPU"
          used={cpuUsed}
          capacity={node.cpu_capacity}
          tone="sky"
        />
        <ResourceBar
          label="Memory"
          used={memUsed}
          capacity={node.memory_capacity}
          unit="GB"
          tone="violet"
        />
      </div>

      {labels.length > 0 ? (
        <div className="mt-4 flex flex-wrap gap-1.5">
          {labels.map(([key, value]) => (
            <span
              key={key}
              className="inline-flex items-center gap-1 rounded bg-slate-100 px-1.5 py-0.5 text-[11px] text-slate-600 dark:bg-slate-800 dark:text-slate-300"
            >
              <Tag className="h-3 w-3" aria-hidden />
              {key}={value}
            </span>
          ))}
        </div>
      ) : null}

      <div className="mt-4 flex items-center justify-between border-t border-slate-100 pt-3 text-xs text-slate-500 dark:border-slate-800 dark:text-slate-400">
        <span>Free: {node.gpu_available} GPU · {formatMemory(node.memory_available)}</span>
        <span>Heartbeat {timeAgo(node.last_heartbeat)}</span>
      </div>
    </Card>
  );
}

export default function NodesPage() {
  const { data, error, isLoading, mutate } = useNodes();
  const nodes = data?.nodes ?? [];

  return (
    <>
      <PageHeader
        title="Nodes"
        description="Fleet inventory with per-node health and resource capacity."
      />

      {error && !data ? (
        <ErrorState error={error} onRetry={() => void mutate()} />
      ) : isLoading && !data ? (
        <LoadingState label="Loading nodes…" />
      ) : nodes.length === 0 ? (
        <EmptyState
          title="No nodes registered"
          description="Nodes will appear here once they join the cluster and send heartbeats."
          icon={Server}
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {nodes.map((node) => (
            <NodeCard key={node.node_id} node={node} />
          ))}
        </div>
      )}
    </>
  );
}
