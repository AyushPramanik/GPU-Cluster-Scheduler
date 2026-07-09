"use client";

import { AlertTriangle } from "lucide-react";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { CreateJobRequest, Job } from "@/lib/types";
import { Button } from "./Button";
import { Modal } from "./Modal";

interface SubmitJobModalProps {
  open: boolean;
  onClose: () => void;
  onSubmitted: (job: Job) => void;
  defaultUserId?: string;
}

interface FormState {
  name: string;
  user_id: string;
  priority: number;
  gpu_count: number;
  cpu_count: number;
  memory_gb: number;
  image: string;
  command: string;
}

const initialForm = (userId: string): FormState => ({
  name: "",
  user_id: userId,
  priority: 5,
  gpu_count: 1,
  cpu_count: 4,
  memory_gb: 16,
  image: "",
  command: "",
});

const labelClass =
  "mb-1 block text-xs font-medium text-slate-600 dark:text-slate-300";
const inputClass =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 shadow-sm outline-none transition-colors placeholder:text-slate-400 focus:border-brand-500 focus:ring-2 focus:ring-brand-500/30 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 dark:placeholder:text-slate-500";

function priorityLabel(priority: number): string {
  if (priority >= 8) return "Critical";
  if (priority >= 6) return "High";
  if (priority >= 4) return "Normal";
  if (priority >= 2) return "Low";
  return "Best-effort";
}

export function SubmitJobModal({
  open,
  onClose,
  onSubmitted,
  defaultUserId = "",
}: SubmitJobModalProps) {
  const [form, setForm] = useState<FormState>(() => initialForm(defaultUserId));
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  function handleClose() {
    if (submitting) return;
    setError(null);
    onClose();
  }

  function numberOr(value: string, fallback: number): number {
    const n = Number(value);
    return Number.isFinite(n) ? n : fallback;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    if (!form.name.trim()) {
      setError("Job name is required.");
      return;
    }
    if (!form.user_id.trim()) {
      setError("User ID is required.");
      return;
    }
    if (!form.image.trim()) {
      setError("Container image is required.");
      return;
    }

    const payload: CreateJobRequest = {
      name: form.name.trim(),
      user_id: form.user_id.trim(),
      priority: form.priority,
      gpu_count: Math.max(0, Math.floor(form.gpu_count)),
      cpu_count: Math.max(0, Math.floor(form.cpu_count)),
      memory_gb: Math.max(0, Math.floor(form.memory_gb)),
      image: form.image.trim(),
      command: form.command.trim(),
    };

    setSubmitting(true);
    try {
      const job = await api.createJob(payload);
      onSubmitted(job);
      setForm(initialForm(defaultUserId));
      onClose();
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : "Failed to submit job. Please try again.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="Submit Job"
      description="Queue a new workload onto the GPU cluster."
      footer={
        <>
          <Button variant="secondary" onClick={handleClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" form="submit-job-form" loading={submitting}>
            Submit Job
          </Button>
        </>
      }
    >
      <form id="submit-job-form" onSubmit={handleSubmit} className="space-y-4">
        {error ? (
          <div className="flex items-start gap-2 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-300">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
            <span>{error}</span>
          </div>
        ) : null}

        <div className="grid grid-cols-2 gap-4">
          <div className="col-span-2 sm:col-span-1">
            <label className={labelClass} htmlFor="job-name">
              Job Name
            </label>
            <input
              id="job-name"
              className={inputClass}
              value={form.name}
              onChange={(e) => update("name", e.target.value)}
              placeholder="resnet-training"
              autoFocus
            />
          </div>
          <div className="col-span-2 sm:col-span-1">
            <label className={labelClass} htmlFor="job-user">
              User ID
            </label>
            <input
              id="job-user"
              className={inputClass}
              value={form.user_id}
              onChange={(e) => update("user_id", e.target.value)}
              placeholder="alice"
            />
          </div>
        </div>

        <div>
          <div className="mb-1 flex items-center justify-between">
            <label className={labelClass} htmlFor="job-priority">
              Priority
            </label>
            <span className="text-xs font-semibold tabular-nums text-brand-600 dark:text-brand-400">
              {form.priority} · {priorityLabel(form.priority)}
            </span>
          </div>
          <input
            id="job-priority"
            type="range"
            min={0}
            max={10}
            step={1}
            value={form.priority}
            onChange={(e) => update("priority", Number(e.target.value))}
            className="w-full accent-brand-600"
          />
          <div className="mt-1 flex justify-between text-[10px] text-slate-400">
            <span>0</span>
            <span>5</span>
            <span>10</span>
          </div>
        </div>

        <div className="grid grid-cols-3 gap-4">
          <div>
            <label className={labelClass} htmlFor="job-gpu">
              GPUs
            </label>
            <input
              id="job-gpu"
              type="number"
              min={0}
              className={inputClass}
              value={form.gpu_count}
              onChange={(e) => update("gpu_count", numberOr(e.target.value, 0))}
            />
          </div>
          <div>
            <label className={labelClass} htmlFor="job-cpu">
              CPUs
            </label>
            <input
              id="job-cpu"
              type="number"
              min={0}
              className={inputClass}
              value={form.cpu_count}
              onChange={(e) => update("cpu_count", numberOr(e.target.value, 0))}
            />
          </div>
          <div>
            <label className={labelClass} htmlFor="job-mem">
              Memory (GB)
            </label>
            <input
              id="job-mem"
              type="number"
              min={0}
              className={inputClass}
              value={form.memory_gb}
              onChange={(e) => update("memory_gb", numberOr(e.target.value, 0))}
            />
          </div>
        </div>

        <div>
          <label className={labelClass} htmlFor="job-image">
            Container Image
          </label>
          <input
            id="job-image"
            className={inputClass}
            value={form.image}
            onChange={(e) => update("image", e.target.value)}
            placeholder="nvcr.io/nvidia/pytorch:24.05-py3"
          />
        </div>

        <div>
          <label className={labelClass} htmlFor="job-command">
            Command
          </label>
          <textarea
            id="job-command"
            className={`${inputClass} min-h-[72px] font-mono text-xs`}
            value={form.command}
            onChange={(e) => update("command", e.target.value)}
            placeholder="python train.py --epochs 100"
          />
        </div>
      </form>
    </Modal>
  );
}
