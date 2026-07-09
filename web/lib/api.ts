// Typed API client for the GPU Cluster Scheduler Go REST API.

import type {
  CancelJobResponse,
  ClusterUtilization,
  CreateJobRequest,
  Job,
  JobsQuery,
  JobsResponse,
  NodesResponse,
  SchedulingEventsResponse,
} from "./types";

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

interface RequestOptions {
  method?: "GET" | "POST" | "DELETE" | "PUT" | "PATCH";
  body?: unknown;
  signal?: AbortSignal;
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, signal } = options;

  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      method,
      signal,
      headers: body ? { "Content-Type": "application/json" } : undefined,
      body: body ? JSON.stringify(body) : undefined,
      cache: "no-store",
    });
  } catch (err) {
    throw new ApiError(
      0,
      err instanceof Error
        ? `Network error: ${err.message}`
        : "Network error contacting the scheduler API",
    );
  }

  if (!response.ok) {
    let message = `Request failed with status ${response.status}`;
    try {
      const data = (await response.json()) as { error?: string; message?: string };
      message = data.error ?? data.message ?? message;
    } catch {
      // Response body was not JSON; keep the default message.
    }
    throw new ApiError(response.status, message);
  }

  // DELETE / empty responses may not carry a body.
  const text = await response.text();
  if (!text) {
    return undefined as T;
  }
  return JSON.parse(text) as T;
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "" && value !== null) {
      search.set(key, String(value));
    }
  }
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

export const api = {
  listJobs(query: JobsQuery = {}, signal?: AbortSignal): Promise<JobsResponse> {
    const qs = buildQuery({
      status: query.status,
      user_id: query.user_id,
      limit: query.limit,
    });
    return request<JobsResponse>(`/api/v1/jobs${qs}`, { signal });
  },

  getJob(id: string, signal?: AbortSignal): Promise<Job> {
    return request<Job>(`/api/v1/jobs/${encodeURIComponent(id)}`, { signal });
  },

  createJob(payload: CreateJobRequest, signal?: AbortSignal): Promise<Job> {
    return request<Job>("/api/v1/jobs", { method: "POST", body: payload, signal });
  },

  cancelJob(id: string, signal?: AbortSignal): Promise<CancelJobResponse> {
    return request<CancelJobResponse>(`/api/v1/jobs/${encodeURIComponent(id)}`, {
      method: "DELETE",
      signal,
    });
  },

  listNodes(signal?: AbortSignal): Promise<NodesResponse> {
    return request<NodesResponse>("/api/v1/nodes", { signal });
  },

  getClusterUtilization(signal?: AbortSignal): Promise<ClusterUtilization> {
    return request<ClusterUtilization>("/api/v1/cluster/utilization", { signal });
  },

  listSchedulingEvents(
    limit?: number,
    signal?: AbortSignal,
  ): Promise<SchedulingEventsResponse> {
    const qs = buildQuery({ limit });
    return request<SchedulingEventsResponse>(`/api/v1/scheduling-events${qs}`, {
      signal,
    });
  },
};
