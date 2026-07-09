"use client";

// SWR-based data hooks with 3s polling for live cluster state.

import useSWR, { type SWRConfiguration } from "swr";
import { api } from "./api";
import type {
  ClusterUtilization,
  JobsQuery,
  JobsResponse,
  NodesResponse,
  SchedulingEventsResponse,
} from "./types";

const POLL_INTERVAL_MS = 3000;

const swrDefaults: SWRConfiguration = {
  refreshInterval: POLL_INTERVAL_MS,
  revalidateOnFocus: true,
  keepPreviousData: true,
  dedupingInterval: 1000,
};

export function useClusterUtilization() {
  return useSWR<ClusterUtilization>(
    "cluster/utilization",
    () => api.getClusterUtilization(),
    swrDefaults,
  );
}

export function useJobs(query: JobsQuery = {}) {
  const key = ["jobs", query.status ?? "", query.user_id ?? "", query.limit ?? ""];
  return useSWR<JobsResponse>(key, () => api.listJobs(query), swrDefaults);
}

export function useNodes() {
  return useSWR<NodesResponse>("nodes", () => api.listNodes(), swrDefaults);
}

export function useSchedulingEvents(limit = 100) {
  return useSWR<SchedulingEventsResponse>(
    ["scheduling-events", limit],
    () => api.listSchedulingEvents(limit),
    swrDefaults,
  );
}
