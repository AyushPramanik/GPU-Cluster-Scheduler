"use client";

import { CalendarClock, Gauge } from "lucide-react";
import { useMemo } from "react";
import { Card, CardHeader } from "@/components/Card";
import { PageHeader } from "@/components/PageHeader";
import { EmptyState, ErrorState, LoadingState } from "@/components/States";
import { formatDateTime, formatLatency, timeAgo } from "@/lib/format";
import { useSchedulingEvents } from "@/lib/hooks";
import type { SchedulingEvent } from "@/lib/types";

const th =
  "px-4 py-2.5 text-left text-xs font-semibold text-slate-500 dark:text-slate-400";
const td = "px-4 py-3 text-sm text-slate-700 dark:text-slate-200 align-top";

function latencyTone(ms: number): string {
  if (ms < 10) return "text-emerald-600 dark:text-emerald-400";
  if (ms < 100) return "text-amber-600 dark:text-amber-400";
  return "text-rose-600 dark:text-rose-400";
}

function EventsTable({ events }: { events: SchedulingEvent[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[820px]">
        <thead className="border-b border-slate-200 dark:border-slate-800">
          <tr>
            <th className={th}>Time</th>
            <th className={th}>Job</th>
            <th className={th}>Selected Node</th>
            <th className={th}>Reason</th>
            <th className={th}>Algorithm</th>
            <th className={`${th} text-right`}>Latency</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 dark:divide-slate-800/70">
          {events.map((event) => (
            <tr
              key={event.event_id}
              className="transition-colors hover:bg-slate-50 dark:hover:bg-slate-800/40"
            >
              <td className={`${td} whitespace-nowrap`}>
                <div className="text-slate-700 dark:text-slate-200">
                  {formatDateTime(event.timestamp)}
                </div>
                <div className="text-[11px] text-slate-400">
                  {timeAgo(event.timestamp)}
                </div>
              </td>
              <td className={td}>
                <span className="font-mono text-xs">{event.job_id}</span>
              </td>
              <td className={td}>
                {event.selected_node ? (
                  <span className="font-mono text-xs">{event.selected_node}</span>
                ) : (
                  <span className="text-slate-400">none</span>
                )}
              </td>
              <td className={`${td} max-w-sm`}>
                <span className="text-slate-600 dark:text-slate-300">
                  {event.scheduling_reason || "—"}
                </span>
              </td>
              <td className={td}>
                <span className="inline-flex items-center gap-1 rounded bg-brand-50 px-1.5 py-0.5 text-[11px] font-medium text-brand-700 dark:bg-brand-500/15 dark:text-brand-300">
                  {event.algorithm || "—"}
                </span>
              </td>
              <td className={`${td} text-right`}>
                <span
                  className={`inline-flex items-center justify-end gap-1 font-semibold tabular-nums ${latencyTone(
                    event.latency_ms,
                  )}`}
                >
                  <Gauge className="h-3.5 w-3.5" aria-hidden />
                  {formatLatency(event.latency_ms)}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default function EventsPage() {
  const { data, error, isLoading, mutate } = useSchedulingEvents(100);

  const events = useMemo(() => {
    const list = data?.events ?? [];
    return [...list].sort(
      (a, b) =>
        new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime(),
    );
  }, [data]);

  const avgLatency = useMemo(() => {
    if (events.length === 0) return 0;
    const total = events.reduce((sum, e) => sum + e.latency_ms, 0);
    return total / events.length;
  }, [events]);

  return (
    <>
      <PageHeader
        title="Scheduling Events"
        description="Placement decisions made by the scheduler, newest first."
      />

      {error && !data ? (
        <ErrorState error={error} onRetry={() => void mutate()} />
      ) : isLoading && !data ? (
        <LoadingState label="Loading scheduling events…" />
      ) : (
        <Card>
          <CardHeader
            title="Decision Timeline"
            description={`${events.length} events · avg latency ${formatLatency(
              avgLatency,
            )}`}
          />
          {events.length === 0 ? (
            <div className="p-4">
              <EmptyState
                title="No scheduling events"
                description="Placement decisions will stream in here as jobs are scheduled."
                icon={CalendarClock}
              />
            </div>
          ) : (
            <EventsTable events={events} />
          )}
        </Card>
      )}
    </>
  );
}
