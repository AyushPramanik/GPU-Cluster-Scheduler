"use client";

import { Boxes, Plus, X } from "lucide-react";
import { useMemo, useState } from "react";
import { Button } from "@/components/Button";
import { Card, CardHeader } from "@/components/Card";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { SubmitJobModal } from "@/components/SubmitJobModal";
import { EmptyState, ErrorState, LoadingState } from "@/components/States";
import { api, ApiError } from "@/lib/api";
import { timeAgo } from "@/lib/format";
import { useJobs } from "@/lib/hooks";
import type { Job, JobStatus } from "@/lib/types";

const CANCELLABLE: ReadonlySet<JobStatus> = new Set([
  "queued",
  "scheduled",
  "running",
]);

const th = "px-4 py-2.5 text-left text-xs font-semibold text-slate-500 dark:text-slate-400";
const td = "px-4 py-3 text-sm text-slate-700 dark:text-slate-200";

function ResourceChips({ job }: { job: Job }) {
  return (
    <div className="flex flex-wrap gap-1">
      <span className="rounded bg-brand-50 px-1.5 py-0.5 text-[11px] font-medium text-brand-700 dark:bg-brand-500/15 dark:text-brand-300">
        {job.gpu_count} GPU
      </span>
      <span className="rounded bg-slate-100 px-1.5 py-0.5 text-[11px] font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300">
        {job.cpu_count} CPU
      </span>
      <span className="rounded bg-slate-100 px-1.5 py-0.5 text-[11px] font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300">
        {job.memory_gb} GB
      </span>
    </div>
  );
}

function JobsTable({
  title,
  description,
  jobs,
  onCancel,
  cancelling,
  emptyLabel,
}: {
  title: string;
  description: string;
  jobs: Job[];
  onCancel: (id: string) => void;
  cancelling: Record<string, boolean>;
  emptyLabel: string;
}) {
  return (
    <Card>
      <CardHeader
        title={title}
        description={description}
        action={
          <span className="rounded-full bg-slate-100 px-2.5 py-0.5 text-xs font-semibold text-slate-600 dark:bg-slate-800 dark:text-slate-300">
            {jobs.length}
          </span>
        }
      />
      {jobs.length === 0 ? (
        <div className="p-4">
          <EmptyState title={emptyLabel} icon={Boxes} />
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[720px]">
            <thead className="border-b border-slate-200 dark:border-slate-800">
              <tr>
                <th className={th}>Job</th>
                <th className={th}>User</th>
                <th className={th}>Status</th>
                <th className={th}>Priority</th>
                <th className={th}>Resources</th>
                <th className={th}>Node</th>
                <th className={th}>Age</th>
                <th className={`${th} text-right`}>Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-800/70">
              {jobs.map((job) => (
                <tr
                  key={job.job_id}
                  className="transition-colors hover:bg-slate-50 dark:hover:bg-slate-800/40"
                >
                  <td className={td}>
                    <div className="font-medium text-slate-900 dark:text-slate-100">
                      {job.name || "—"}
                    </div>
                    <div className="font-mono text-[11px] text-slate-400">
                      {job.job_id}
                    </div>
                  </td>
                  <td className={td}>{job.user_id}</td>
                  <td className={td}>
                    <StatusBadge status={job.status} />
                  </td>
                  <td className={`${td} tabular-nums`}>{job.priority}</td>
                  <td className={td}>
                    <ResourceChips job={job} />
                  </td>
                  <td className={td}>
                    {job.node_id ? (
                      <span className="font-mono text-xs">{job.node_id}</span>
                    ) : (
                      <span className="text-slate-400">unassigned</span>
                    )}
                  </td>
                  <td className={`${td} whitespace-nowrap text-slate-500`}>
                    {timeAgo(job.created_at)}
                    {job.retry_count > 0 ? (
                      <span className="ml-1.5 rounded bg-amber-100 px-1 text-[10px] font-medium text-amber-700 dark:bg-amber-500/15 dark:text-amber-300">
                        retry {job.retry_count}
                      </span>
                    ) : null}
                  </td>
                  <td className={`${td} text-right`}>
                    {CANCELLABLE.has(job.status) ? (
                      <Button
                        variant="danger"
                        size="sm"
                        loading={cancelling[job.job_id]}
                        onClick={() => onCancel(job.job_id)}
                      >
                        {!cancelling[job.job_id] ? (
                          <X className="h-3.5 w-3.5" aria-hidden />
                        ) : null}
                        Cancel
                      </Button>
                    ) : (
                      <span className="text-xs text-slate-400">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}

export default function JobsPage() {
  const { data, error, isLoading, mutate } = useJobs({ limit: 200 });
  const [modalOpen, setModalOpen] = useState(false);
  const [cancelling, setCancelling] = useState<Record<string, boolean>>({});
  const [actionError, setActionError] = useState<string | null>(null);

  const jobs = useMemo(() => data?.jobs ?? [], [data]);

  const { queued, active, finished } = useMemo(() => {
    const q: Job[] = [];
    const a: Job[] = [];
    const f: Job[] = [];
    for (const job of jobs) {
      if (job.status === "queued") q.push(job);
      else if (job.status === "running" || job.status === "scheduled") a.push(job);
      else f.push(job);
    }
    q.sort((x, y) => y.priority - x.priority);
    return { queued: q, active: a, finished: f };
  }, [jobs]);

  async function handleCancel(id: string) {
    setActionError(null);
    setCancelling((prev) => ({ ...prev, [id]: true }));
    try {
      await api.cancelJob(id);
      await mutate();
    } catch (err) {
      setActionError(
        err instanceof ApiError ? err.message : "Failed to cancel job.",
      );
    } finally {
      setCancelling((prev) => {
        const next = { ...prev };
        delete next[id];
        return next;
      });
    }
  }

  return (
    <>
      <PageHeader
        title="Jobs"
        description="Submit, monitor, and cancel workloads across the cluster."
        action={
          <Button onClick={() => setModalOpen(true)}>
            <Plus className="h-4 w-4" aria-hidden />
            Submit Job
          </Button>
        }
      />

      {actionError ? (
        <div className="mb-4 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-300">
          {actionError}
        </div>
      ) : null}

      {error && !data ? (
        <ErrorState error={error} onRetry={() => void mutate()} />
      ) : isLoading && !data ? (
        <LoadingState label="Loading jobs…" />
      ) : (
        <div className="space-y-6">
          <JobsTable
            title="Active Jobs"
            description="Scheduled and running workloads."
            jobs={active}
            onCancel={handleCancel}
            cancelling={cancelling}
            emptyLabel="No active jobs"
          />
          <JobsTable
            title="Queued Jobs"
            description="Awaiting scheduling, ordered by priority."
            jobs={queued}
            onCancel={handleCancel}
            cancelling={cancelling}
            emptyLabel="Queue is empty"
          />
          <JobsTable
            title="Recently Finished"
            description="Completed, failed, cancelled, and preempted jobs."
            jobs={finished}
            onCancel={handleCancel}
            cancelling={cancelling}
            emptyLabel="No finished jobs yet"
          />
        </div>
      )}

      <SubmitJobModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onSubmitted={() => void mutate()}
        defaultUserId="ayush@plainvue.ai"
      />
    </>
  );
}
