// SPDX-License-Identifier: Apache-2.0

// API utility functions for HyperSDK

const API_BASE = ''; // No /api prefix - endpoints already include it

export async function fetchAPI<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${endpoint}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    ...options,
  });

  if (!response.ok) {
    throw new Error(`API error: ${response.status} ${response.statusText}`);
  }

  return response.json();
}

export async function getHealth() {
  return fetchAPI<{ status: string; version: string }>('/health');
}

export async function getStatus() {
  return fetchAPI('/status');
}

export async function getCapabilities() {
  return fetchAPI('/capabilities');
}

export async function submitJob(jobDefinition: unknown) {
  return fetchAPI('/jobs/submit', {
    method: 'POST',
    body: JSON.stringify(jobDefinition),
  });
}

export async function queryJobs(query?: string) {
  const endpoint = query ? `/jobs/query?${query}` : '/jobs/query';
  return fetchAPI(endpoint);
}

export async function cancelJob(jobId: string) {
  return fetchAPI(`/jobs/cancel?id=${jobId}`, {
    method: 'POST',
  });
}

export async function getJob(jobId: string) {
  return fetchAPI(`/jobs/${jobId}`);
}

export interface NutanixConnection {
  server: string;
  username: string;
  password: string;
  cluster?: string;
  insecure?: boolean;
}

export async function listNutanixVMs(conn: NutanixConnection) {
  const params = new URLSearchParams({
    provider: 'nutanix',
    server: conn.server,
    username: conn.username,
    password: conn.password,
    insecure: String(conn.insecure ?? false),
  });
  if (conn.cluster) params.set('cluster', conn.cluster);

  const response = await fetch(`/vms/list?${params.toString()}`, { method: 'GET' });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Failed to list Nutanix VMs (${response.status})`);
  }
  return response.json() as Promise<{ vms: unknown[]; total: number }>;
}

export async function getNutanixVMInfo(conn: NutanixConnection, vm: string) {
  const params = new URLSearchParams({
    provider: 'nutanix',
    vm,
    server: conn.server,
    username: conn.username,
    password: conn.password,
    insecure: String(conn.insecure ?? false),
  });
  if (conn.cluster) params.set('cluster', conn.cluster);

  const response = await fetch(`/vms/info?${params.toString()}`, { method: 'GET' });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Failed to get VM info (${response.status})`);
  }
  return response.json();
}

export async function submitNutanixExportJob(jobDefinition: unknown) {
  return fetchAPI('/jobs/submit', {
    method: 'POST',
    body: JSON.stringify(jobDefinition),
  });
}

export async function exportNutanixVMSync(body: unknown) {
  const response = await fetch('/vms/export?provider=nutanix', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Export failed (${response.status})`);
  }
  return response.json();
}

export async function listSchedules() {
  return fetchAPI('/schedules');
}

export async function createSchedule(schedule: unknown) {
  return fetchAPI('/schedules', {
    method: 'POST',
    body: JSON.stringify(schedule),
  });
}

export async function updateSchedule(id: string, schedule: unknown) {
  return fetchAPI(`/schedules/${id}`, {
    method: 'PUT',
    body: JSON.stringify(schedule),
  });
}

export async function deleteSchedule(id: string) {
  return fetchAPI(`/schedules/${id}`, {
    method: 'DELETE',
  });
}

export async function enableSchedule(id: string) {
  return fetchAPI(`/schedules/${id}/enable`, {
    method: 'POST',
  });
}

export async function disableSchedule(id: string) {
  return fetchAPI(`/schedules/${id}/disable`, {
    method: 'POST',
  });
}

export async function triggerSchedule(id: string) {
  return fetchAPI(`/schedules/${id}/trigger`, {
    method: 'POST',
  });
}

export async function listWebhooks() {
  return fetchAPI('/webhooks');
}

export async function addWebhook(webhook: unknown) {
  return fetchAPI('/webhooks', {
    method: 'POST',
    body: JSON.stringify(webhook),
  });
}

export async function deleteWebhook(id: string) {
  return fetchAPI(`/webhooks/${id}`, {
    method: 'DELETE',
  });
}

export async function testWebhook(url: string) {
  return fetchAPI('/webhooks/test', {
    method: 'POST',
    body: JSON.stringify({ url }),
  });
}
