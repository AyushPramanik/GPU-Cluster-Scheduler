// k6 load test for the GPU Cluster Scheduler API.
//
// Usage:
//   k6 run scripts/loadtest.js
//   API_URL=http://localhost:8080 k6 run --vus 100 --duration 2m scripts/loadtest.js
//
// It exercises the hot paths: submitting jobs, listing jobs, and reading cluster
// utilization, while asserting latency SLOs.
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const API = __ENV.API_URL || 'http://localhost:8080';

const submitErrors = new Rate('submit_errors');
const submitLatency = new Trend('submit_latency_ms', true);

export const options = {
  scenarios: {
    submit_jobs: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '15s', target: 25 },
        { duration: '60s', target: 50 },
        { duration: '15s', target: 0 },
      ],
      exec: 'submitJobs',
    },
    read_cluster: {
      executor: 'constant-vus',
      vus: 10,
      duration: '90s',
      exec: 'readCluster',
    },
  },
  thresholds: {
    submit_errors: ['rate<0.05'],
    submit_latency_ms: ['p(95)<250'],
    http_req_duration: ['p(99)<500'],
  },
};

const IMAGES = ['pytorch/pytorch:2.3.0-cuda12.1', 'tensorflow/tensorflow:2.16-gpu', 'nvcr.io/nvidia/tritonserver:24.05'];
const TEAMS = ['research', 'inference', 'platform'];

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

export function submitJobs() {
  const payload = JSON.stringify({
    name: `train-${Math.floor(Math.random() * 1e6)}`,
    user_id: `user-${Math.floor(Math.random() * 50)}`,
    team_id: pick(TEAMS),
    priority: Math.floor(Math.random() * 11),
    gpu_count: 1 + Math.floor(Math.random() * 4),
    cpu_count: 4 + Math.floor(Math.random() * 12),
    memory_gb: 16 + Math.floor(Math.random() * 112),
    image: pick(IMAGES),
    command: 'python train.py --epochs 10',
  });

  const res = http.post(`${API}/api/v1/jobs`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  submitLatency.add(res.timings.duration);
  const ok = check(res, { 'job created (201)': (r) => r.status === 201 });
  submitErrors.add(!ok);
  sleep(Math.random() * 0.5);
}

export function readCluster() {
  const util = http.get(`${API}/api/v1/cluster/utilization`);
  check(util, { 'utilization 200': (r) => r.status === 200 });

  const jobs = http.get(`${API}/api/v1/jobs?limit=50`);
  check(jobs, { 'jobs 200': (r) => r.status === 200 });

  const nodes = http.get(`${API}/api/v1/nodes`);
  check(nodes, { 'nodes 200': (r) => r.status === 200 });

  sleep(1);
}
