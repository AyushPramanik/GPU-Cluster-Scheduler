// Formatting helpers shared across the dashboard.

export function formatPercent(ratio: number, digits = 0): string {
  if (!Number.isFinite(ratio)) return "0%";
  return `${(ratio * 100).toFixed(digits)}%`;
}

export function formatNumber(value: number): string {
  return new Intl.NumberFormat("en-US").format(value);
}

export function formatMemory(gb: number): string {
  if (gb >= 1024) {
    return `${(gb / 1024).toFixed(1)} TB`;
  }
  return `${formatNumber(gb)} GB`;
}

// Renders a relative "time ago" label from an ISO timestamp.
export function timeAgo(iso: string | null | undefined): string {
  if (!iso) return "—";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "—";

  const seconds = Math.floor((Date.now() - then) / 1000);
  if (seconds < 0) return "just now";
  if (seconds < 60) return `${seconds}s ago`;

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;

  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;

  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;

  return `${Math.floor(months / 12)}y ago`;
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export function formatLatency(ms: number): string {
  if (!Number.isFinite(ms)) return "—";
  if (ms < 1) return `${(ms * 1000).toFixed(0)}µs`;
  if (ms < 1000) return `${ms.toFixed(ms < 10 ? 1 : 0)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

// Truncates the middle of long identifiers for compact display.
export function shortId(id: string, head = 8): string {
  if (!id) return "—";
  if (id.length <= head) return id;
  return id.slice(0, head);
}
