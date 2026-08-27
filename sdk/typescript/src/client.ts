// SPDX-License-Identifier: Apache-2.0

/**
 * HyperSDK TypeScript client
 */

import {
  Job,
  JobDefinition,
  JobStatus,
  JobProgress,
  QueryRequest,
  QueryResponse,
  SubmitResponse,
  CancelRequest,
  CancelResponse,
  DaemonStatus,
  ScheduledJob,
  Webhook,
  HealthResponse,
  CapabilitiesResponse,
  CarbonStatus,
  CarbonForecast,
  CarbonReport,
  CarbonZone,
  CarbonEstimate,
} from './models';

import {
  HyperSDKError,
  AuthenticationError,
  JobNotFoundError,
  APIError,
} from './errors';

export interface HyperSDKConfig {
  baseURL: string;
  apiKey?: string;
  timeout?: number;
  headers?: Record<string, string>;
}

export class HyperSDK {
  private baseURL: string;
  private apiKey?: string;
  private timeout: number;
  private headers: Record<string, string>;
  private token?: string;

  constructor(config: HyperSDKConfig | string) {
    if (typeof config === 'string') {
      this.baseURL = config.replace(/\/$/, '');
      this.timeout = 30000;
      this.headers = {};
    } else {
      this.baseURL = config.baseURL.replace(/\/$/, '');
      this.apiKey = config.apiKey;
      this.timeout = config.timeout || 30000;
      this.headers = config.headers || {};
    }

    if (this.apiKey) {
      this.headers['X-API-Key'] = this.apiKey;
    }
  }

  private buildURL(path: string): string {
    return `${this.baseURL}${path}`;
  }

  private async request<T>(
    method: string,
    path: string,
    options: {
      body?: any;
      params?: Record<string, string>;
      headers?: Record<string, string>;
    } = {}
  ): Promise<T> {
    const url = new URL(this.buildURL(path));

    if (options.params) {
      Object.entries(options.params).forEach(([key, value]) => {
        url.searchParams.append(key, value);
      });
    }

    const headers: Record<string, string> = {
      ...this.headers,
      ...options.headers,
    };

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    if (options.body) {
      headers['Content-Type'] = 'application/json';
    }

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const response = await fetch(url.toString(), {
        method,
        headers,
        body: options.body ? JSON.stringify(options.body) : undefined,
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      if (response.status === 404) {
        throw new JobNotFoundError(`Resource not found: ${path}`);
      }

      if (response.status === 401) {
        throw new AuthenticationError('Authentication failed');
      }

      if (!response.ok) {
        let errorMessage = response.statusText;
        let errorData: any;

        try {
          errorData = await response.json();
          errorMessage = errorData.error || errorMessage;
        } catch (e) {
          // Ignore JSON parse errors
        }

        throw new APIError(
          `API error: ${errorMessage}`,
          response.status,
          errorData
        );
      }

      const contentType = response.headers.get('content-type');
      if (contentType && contentType.includes('application/json')) {
        return await response.json();
      } else if (contentType && contentType.includes('text/')) {
        return (await response.text()) as any;
      }

      return null as any;
    } catch (error) {
      clearTimeout(timeoutId);

      if (error instanceof HyperSDKError) {
        throw error;
      }

      if (error instanceof Error && error.name === 'AbortError') {
        throw new APIError('Request timeout');
      }

      throw new APIError(`Request failed: ${error}`);
    }
  }

  // Authentication

  async login(username: string, password: string): Promise<string> {
    const response = await this.request<{ token: string; expires_at: string }>(
      'POST',
      '/api/login',
      {
        body: { username, password },
      }
    );
    this.token = response.token;
    return this.token;
  }

  async logout(): Promise<void> {
    await this.request('POST', '/api/logout');
    this.token = undefined;
  }

  // Health & Status

  async health(): Promise<HealthResponse> {
    return this.request<HealthResponse>('GET', '/health');
  }

  async status(): Promise<DaemonStatus> {
    return this.request<DaemonStatus>('GET', '/status');
  }

  async capabilities(): Promise<CapabilitiesResponse> {
    return this.request<CapabilitiesResponse>('GET', '/capabilities');
  }

  // Job Management

  async submitJob(jobDef: JobDefinition): Promise<string> {
    const response = await this.request<SubmitResponse>('POST', '/jobs/submit', {
      body: jobDef,
    });

    if (response.accepted === 0) {
      const error = response.errors?.[0] || 'Unknown error';
      throw new APIError(`Job submission failed: ${error}`);
    }

    return response.job_ids[0];
  }

  async submitJobs(jobDefs: JobDefinition[]): Promise<string[]> {
    const response = await this.request<SubmitResponse>('POST', '/jobs/submit', {
      body: jobDefs,
    });
    return response.job_ids;
  }

  async getJob(jobId: string): Promise<Job> {
    return this.request<Job>('GET', `/jobs/${jobId}`);
  }

  async queryJobs(query: QueryRequest = {}): Promise<Job[]> {
    const response = await this.request<QueryResponse>('POST', '/jobs/query', {
      body: query,
    });
    return response.jobs;
  }

  async listJobs(all: boolean = true): Promise<Job[]> {
    const response = await this.request<QueryResponse>('GET', '/jobs/query', {
      params: all ? { all: 'true' } : {},
    });
    return response.jobs;
  }

  async cancelJob(jobId: string): Promise<boolean> {
    const response = await this.request<CancelResponse>('POST', '/jobs/cancel', {
      body: { job_ids: [jobId] },
    });

    if (response.cancelled.includes(jobId)) {
      return true;
    }

    if (response.failed.includes(jobId)) {
      const error = response.errors?.[jobId] || 'Unknown error';
      throw new APIError(`Failed to cancel job: ${error}`);
    }

    return false;
  }

  async cancelJobs(jobIds: string[]): Promise<CancelResponse> {
    return this.request<CancelResponse>('POST', '/jobs/cancel', {
      body: { job_ids: jobIds },
    });
  }

  async getJobProgress(jobId: string): Promise<JobProgress> {
    return this.request<JobProgress>('GET', `/jobs/progress/${jobId}`);
  }

  async getJobLogs(jobId: string): Promise<string> {
    return this.request<string>('GET', `/jobs/logs/${jobId}`);
  }

  async getJobETA(jobId: string): Promise<string> {
    const response = await this.request<{ eta?: string; estimated_remaining?: string }>(
      'GET',
      `/jobs/eta/${jobId}`
    );
    return response.estimated_remaining || response.eta || 'Unknown';
  }

  // VM Operations

  async listVMs(vcenterConfig: any): Promise<any[]> {
    return this.request<any[]>('POST', '/vms/list', {
      body: vcenterConfig,
    });
  }

  async getVMInfo(vcenterConfig: any, vmPath: string): Promise<any> {
    return this.request<any>('POST', '/vms/info', {
      body: { vcenter: vcenterConfig, vm_path: vmPath },
    });
  }

  async shutdownVM(vcenterConfig: any, vmPath: string): Promise<any> {
    return this.request<any>('POST', '/vms/shutdown', {
      body: { vcenter: vcenterConfig, vm_path: vmPath },
    });
  }

  // Schedule Management

  async listSchedules(): Promise<ScheduledJob[]> {
    return this.request<ScheduledJob[]>('GET', '/schedules');
  }

  async createSchedule(schedule: ScheduledJob): Promise<ScheduledJob> {
    return this.request<ScheduledJob>('POST', '/schedules', {
      body: schedule,
    });
  }

  async getSchedule(scheduleId: string): Promise<ScheduledJob> {
    return this.request<ScheduledJob>('GET', `/schedules/${scheduleId}`);
  }

  async updateSchedule(
    scheduleId: string,
    schedule: ScheduledJob
  ): Promise<ScheduledJob> {
    return this.request<ScheduledJob>('PUT', `/schedules/${scheduleId}`, {
      body: schedule,
    });
  }

  async deleteSchedule(scheduleId: string): Promise<void> {
    await this.request('DELETE', `/schedules/${scheduleId}`);
  }

  async enableSchedule(scheduleId: string): Promise<void> {
    await this.request('POST', `/schedules/${scheduleId}/enable`);
  }

  async disableSchedule(scheduleId: string): Promise<void> {
    await this.request('POST', `/schedules/${scheduleId}/disable`);
  }

  async triggerSchedule(scheduleId: string): Promise<void> {
    await this.request('POST', `/schedules/${scheduleId}/trigger`);
  }

  // Webhook Management

  async listWebhooks(): Promise<Webhook[]> {
    return this.request<Webhook[]>('GET', '/webhooks');
  }

  async addWebhook(webhook: Webhook): Promise<void> {
    await this.request('POST', '/webhooks', { body: webhook });
  }

  async testWebhook(url: string): Promise<void> {
    await this.request('POST', '/webhooks/test', { body: { url } });
  }

  async deleteWebhook(webhookId: string): Promise<void> {
    await this.request('DELETE', `/webhooks/${webhookId}`);
  }

  // Libvirt Operations

  async listDomains(): Promise<any[]> {
    return this.request<any[]>('GET', '/libvirt/domains');
  }

  async getDomain(name: string): Promise<any> {
    return this.request<any>('GET', '/libvirt/domain', {
      params: { name },
    });
  }

  async startDomain(name: string): Promise<void> {
    await this.request('POST', '/libvirt/domain/start', {
      body: { name },
    });
  }

  async shutdownDomain(name: string): Promise<void> {
    await this.request('POST', '/libvirt/domain/shutdown', {
      body: { name },
    });
  }

  async listSnapshots(domain: string): Promise<any[]> {
    return this.request<any[]>('GET', '/libvirt/snapshots', {
      params: { domain },
    });
  }

  async createSnapshot(
    domain: string,
    name: string,
    description?: string
  ): Promise<void> {
    await this.request('POST', '/libvirt/snapshot/create', {
      body: { domain, name, description },
    });
  }

  // Cost Estimation

  async estimateCost(params: {
    provider: string;
    region: string;
    storageClass: string;
    storageGB: number;
    transferGB?: number;
    requests?: number;
    durationDays?: number;
  }): Promise<any> {
    return this.request('POST', '/cost/estimate', {
      body: {
        provider: params.provider,
        region: params.region,
        storage_class: params.storageClass,
        storage_gb: params.storageGB,
        transfer_gb: params.transferGB || 0,
        requests: params.requests || 0,
        duration_days: params.durationDays || 30,
      },
    });
  }

  async compareProviders(params: {
    storageGB: number;
    transferGB?: number;
    requests?: number;
    durationDays?: number;
  }): Promise<any> {
    return this.request('POST', '/cost/compare', {
      body: {
        storage_gb: params.storageGB,
        transfer_gb: params.transferGB || 0,
        requests: params.requests || 0,
        duration_days: params.durationDays || 30,
      },
    });
  }

  async projectYearlyCost(params: {
    provider: string;
    storageClass: string;
    storageGB: number;
    transferGB?: number;
    requests?: number;
  }): Promise<any> {
    return this.request('POST', '/cost/project', {
      body: {
        provider: params.provider,
        storage_class: params.storageClass,
        storage_gb: params.storageGB,
        transfer_gb: params.transferGB || 0,
        requests: params.requests || 0,
        duration_days: 30,
      },
    });
  }

  async estimateExportSize(params: {
    diskSizeGB: number;
    format?: string;
    includeSnapshots?: boolean;
  }): Promise<any> {
    return this.request('POST', '/cost/estimate-size', {
      body: {
        disk_size_gb: params.diskSizeGB,
        format: params.format || 'ova',
        include_snapshots: params.includeSnapshots || false,
      },
    });
  }

  // Advanced Scheduling

  async createAdvancedSchedule(params: {
    name: string;
    schedule: string;
    jobTemplate: any;
    description?: string;
    advancedConfig?: {
      depends_on?: Array<{
        job_id: string;
        required_state: string;
        timeout?: number;
      }>;
      retry_policy?: {
        max_attempts: number;
        initial_delay: number;
        max_delay: number;
        backoff_strategy: string;
        retry_on_errors?: string[];
      };
      time_windows?: Array<{
        start_time: string;
        end_time: string;
        days: string[];
        timezone: string;
      }>;
      priority?: number;
      conditions?: any[];
      max_concurrent?: number;
      skip_if_running?: boolean;
      notify_on_start?: boolean;
      notify_on_success?: boolean;
      notify_on_failure?: boolean;
      notify_on_retry?: boolean;
    };
  }): Promise<{
    success: boolean;
    message: string;
    schedule: any;
  }> {
    return this.request('POST', '/schedules/advanced/create', {
      body: {
        name: params.name,
        description: params.description || '',
        schedule: params.schedule,
        job_template: params.jobTemplate,
        advanced_config: params.advancedConfig,
      },
    });
  }

  async getDependencyStatus(jobId: string): Promise<{
    job_id: string;
    job_name: string;
    satisfied: boolean;
    reason?: string;
    dependencies: any[];
    waiting_jobs?: string[];
  }> {
    return this.request('GET', '/schedules/dependencies', {
      params: { job_id: jobId },
    });
  }

  async getRetryStatus(jobId: string): Promise<{
    job_id: string;
    job_name: string;
    attempt: number;
    max_attempts: number;
    last_error?: string;
    next_retry?: string;
    history?: any[];
  }> {
    return this.request('GET', '/schedules/retry', {
      params: { job_id: jobId },
    });
  }

  async getTimeWindowStatus(jobId: string): Promise<{
    job_id: string;
    job_name: string;
    in_window: boolean;
    message: string;
    next_window_start?: string;
    windows?: any[];
  }> {
    return this.request('GET', '/schedules/timewindow', {
      params: { job_id: jobId },
    });
  }

  async getJobQueueStatus(): Promise<{
    queue_size: number;
    running_jobs: number;
    max_slots: number;
    queued_jobs: any[];
  }> {
    return this.request('GET', '/schedules/queue');
  }

  async validateSchedule(params: {
    name: string;
    schedule: string;
    jobTemplate: any;
    advancedConfig?: any;
  }): Promise<{
    valid: boolean;
    errors?: string[];
    message?: string;
  }> {
    return this.request('POST', '/schedules/validate', {
      body: {
        name: params.name,
        schedule: params.schedule,
        job_template: params.jobTemplate,
        advanced_config: params.advancedConfig,
      },
    });
  }

  // Incremental Export & Changed Block Tracking

  async enableCBT(vmPath: string): Promise<{ success: boolean; message: string; error?: string }> {
    return this.request('POST', '/cbt/enable', {
      body: { vm_path: vmPath },
    });
  }

  async disableCBT(vmPath: string): Promise<{ success: boolean; message: string; error?: string }> {
    return this.request('POST', '/cbt/disable', {
      body: { vm_path: vmPath },
    });
  }

  async getCBTStatus(vmPath: string): Promise<{
    vm_path: string;
    cbt_enabled: boolean;
    disks: any[];
    last_export?: any;
    can_incremental: boolean;
    reason?: string;
  }> {
    return this.request('POST', '/cbt/status', {
      body: { vm_path: vmPath },
    });
  }

  async analyzeIncrementalExport(vmPath: string): Promise<{
    vm_path: string;
    can_incremental: boolean;
    reason: string;
    last_export?: any;
    current_disks: any[];
    estimated_savings_bytes: number;
    estimated_duration: string;
  }> {
    return this.request('POST', '/incremental/analyze', {
      body: { vm_path: vmPath },
    });
  }

  // H2KVM Integration

  async convertVM(sourcePath: string, outputPath: string): Promise<string> {
    const response = await this.request<{ conversion_id: string }>(
      'POST',
      '/convert/vm',
      {
        body: { source_path: sourcePath, output_path: outputPath },
      }
    );
    return response.conversion_id;
  }

  async getConversionStatus(conversionId: string): Promise<any> {
    return this.request<any>('GET', '/convert/status', {
      params: { conversion_id: conversionId },
    });
  }

  // Carbon-Aware Scheduling

  /**
   * Get current grid carbon status for a zone.
   *
   * @param zone - Carbon zone ID (default: "US-CAL-CISO")
   * @param threshold - Carbon intensity threshold in gCO2/kWh (default: 200.0)
   * @returns Current carbon status with forecast
   *
   * @example
   * ```typescript
   * const status = await client.getCarbonStatus('US-CAL-CISO', 200);
   * console.log(`Intensity: ${status.current_intensity} gCO2/kWh`);
   * console.log(`Quality: ${status.quality}`);
   * console.log(`Optimal: ${status.optimal_for_backup}`);
   * ```
   */
  async getCarbonStatus(
    zone: string = 'US-CAL-CISO',
    threshold: number = 200.0
  ): Promise<CarbonStatus> {
    return this.request<CarbonStatus>('POST', '/carbon/status', {
      body: { zone, threshold },
    });
  }

  /**
   * List all available carbon zones.
   *
   * @returns Array of available carbon zones (12 global zones)
   *
   * @example
   * ```typescript
   * const zones = await client.listCarbonZones();
   * for (const zone of zones) {
   *   console.log(`${zone.id}: ${zone.name} (${zone.typical_intensity} gCO2/kWh)`);
   * }
   * ```
   */
  async listCarbonZones(): Promise<CarbonZone[]> {
    const response = await this.request<{ zones: CarbonZone[] }>(
      'GET',
      '/carbon/zones'
    );
    return response.zones;
  }

  /**
   * Estimate carbon savings from delaying a backup.
   *
   * @param zone - Carbon zone ID
   * @param dataSizeGB - Data size in GB
   * @param durationHours - Estimated duration in hours (default: 2.0)
   * @returns Carbon savings estimate with run now vs run later comparison
   *
   * @example
   * ```typescript
   * const estimate = await client.estimateCarbonSavings('US-CAL-CISO', 500, 2);
   * console.log(`Run Now: ${estimate.current_emissions_kg_co2} kg CO2`);
   * console.log(`Run Later: ${estimate.best_emissions_kg_co2} kg CO2`);
   * console.log(`Savings: ${estimate.savings_percent}%`);
   * ```
   */
  async estimateCarbonSavings(
    zone: string,
    dataSizeGB: number,
    durationHours: number = 2.0
  ): Promise<CarbonEstimate> {
    return this.request<CarbonEstimate>('POST', '/carbon/estimate', {
      body: {
        zone,
        data_size_gb: dataSizeGB,
        duration_hours: durationHours,
      },
    });
  }

  /**
   * Generate carbon footprint report for a completed job.
   *
   * @param jobId - Job ID
   * @param startTime - Job start time (ISO 8601 format)
   * @param endTime - Job end time (ISO 8601 format)
   * @param dataSizeGB - Data size in GB
   * @param zone - Carbon zone ID (default: "US-CAL-CISO")
   * @returns Carbon footprint report with emissions and savings
   *
   * @example
   * ```typescript
   * const report = await client.getCarbonReport(
   *   'job-123',
   *   '2026-02-04T10:00:00Z',
   *   '2026-02-04T12:00:00Z',
   *   500,
   *   'US-CAL-CISO'
   * );
   * console.log(`Energy: ${report.energy_kwh} kWh`);
   * console.log(`Emissions: ${report.carbon_emissions_kg_co2} kg CO2`);
   * ```
   */
  async getCarbonReport(
    jobId: string,
    startTime: string,
    endTime: string,
    dataSizeGB: number,
    zone: string = 'US-CAL-CISO'
  ): Promise<CarbonReport> {
    return this.request<CarbonReport>('POST', '/carbon/report', {
      body: {
        job_id: jobId,
        start_time: startTime,
        end_time: endTime,
        data_size_gb: dataSizeGB,
        zone,
      },
    });
  }

  /**
   * Submit a carbon-aware job that will be delayed if grid is dirty.
   *
   * @param jobDef - Job definition
   * @param carbonZone - Carbon zone ID (default: "US-CAL-CISO")
   * @param maxIntensity - Maximum carbon intensity threshold in gCO2/kWh (default: 200.0)
   * @param maxDelayHours - Maximum delay allowed in hours (default: 4.0)
   * @returns Job ID
   *
   * @example
   * ```typescript
   * const jobDef = {
   *   vm_path: '/datacenter/vm/prod-db',
   *   output_dir: '/backups'
   * };
   *
   * const jobId = await client.submitCarbonAwareJob(
   *   jobDef,
   *   'US-CAL-CISO',
   *   200,
   *   4
   * );
   * console.log(`Job ID: ${jobId}`);
   * // If grid is dirty, job will be automatically delayed
   * ```
   */
  async submitCarbonAwareJob(
    jobDef: JobDefinition,
    carbonZone: string = 'US-CAL-CISO',
    maxIntensity: number = 200.0,
    maxDelayHours: number = 4.0
  ): Promise<string> {
    // Add carbon-aware metadata to job definition
    const carbonAwareJobDef = {
      ...jobDef,
      metadata: {
        ...(jobDef as any).metadata,
        carbon_aware: true,
        carbon_zone: carbonZone,
        carbon_max_intensity: maxIntensity,
        carbon_max_delay: maxDelayHours * 3600 * 1_000_000_000, // Convert to nanoseconds
      },
    };

    const response = await this.request<SubmitResponse>(
      'POST',
      '/jobs/submit',
      {
        body: carbonAwareJobDef,
      }
    );

    if (response.accepted === 0) {
      const error = response.errors?.[0] || 'Unknown error';
      throw new APIError(`Job submission failed: ${error}`);
    }

    return response.job_ids[0];
  }
}

export default HyperSDK;
