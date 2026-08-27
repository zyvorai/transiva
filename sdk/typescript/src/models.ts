// SPDX-License-Identifier: Apache-2.0

/**
 * HyperSDK data models
 */

export enum JobStatus {
  PENDING = 'pending',
  RUNNING = 'running',
  COMPLETED = 'completed',
  FAILED = 'failed',
  CANCELLED = 'cancelled',
}

export enum ExportFormat {
  QCOW2 = 'qcow2',
  RAW = 'raw',
  VMDK = 'vmdk',
  OVA = 'ova',
  OVF = 'ovf',
}

export enum ExportMethod {
  CTL = 'ctl',
  GOVC = 'govc',
  OVFTOOL = 'ovftool',
  WEB = 'web',
  AUTO = '',
}

export interface VCenterConfig {
  server: string;
  username: string;
  password: string;
  insecure?: boolean;
}

export interface ExportOptions {
  parallel_downloads?: number;
  remove_cdrom?: boolean;
  show_individual_progress?: boolean;
  enable_pipeline?: boolean;
  h2kvm_path?: string;
  /** @deprecated use h2kvm_path */
  hyper2kvm_path?: string;
  pipeline_inspect?: boolean;
  pipeline_fix?: boolean;
  pipeline_convert?: boolean;
  pipeline_validate?: boolean;
  pipeline_compress?: boolean;
  compress_level?: number;
  libvirt_integration?: boolean;
  libvirt_uri?: string;
  libvirt_autostart?: boolean;
  libvirt_bridge?: string;
  libvirt_pool?: string;
}

export interface JobDefinition {
  vm_path: string;
  name?: string;
  id?: string;
  output_path?: string;
  output_dir?: string;
  vcenter_url?: string;
  vcenter?: VCenterConfig;
  username?: string;
  datacenter?: string;
  format?: ExportFormat;
  export_method?: ExportMethod;
  method?: string;
  compress?: boolean;
  thin?: boolean;
  insecure?: boolean;
  options?: ExportOptions;
  created_at?: string;
}

export interface JobProgress {
  phase: string;
  current_file?: string;
  current_step?: string;
  files_downloaded: number;
  total_files: number;
  bytes_downloaded: number;
  bytes_transferred: number;
  total_bytes: number;
  percent_complete: number;
  estimated_remaining?: string;
  export_method?: string;
}

export interface JobResult {
  vm_name: string;
  output_dir: string;
  ovf_path: string;
  files: string[];
  output_files?: string[];
  total_size: number;
  duration: number;
  success: boolean;
  export_method?: string;
  error?: string;
}

export interface Job {
  definition: JobDefinition;
  status: JobStatus;
  progress?: JobProgress;
  result?: JobResult;
  error?: string;
  started_at?: string;
  completed_at?: string;
  updated_at: string;
}

export interface QueryRequest {
  job_ids?: string[];
  status?: JobStatus[];
  all?: boolean;
  limit?: number;
}

export interface QueryResponse {
  jobs: Job[];
  total: number;
  timestamp: string;
}

export interface SubmitResponse {
  job_ids: string[];
  accepted: number;
  rejected: number;
  errors?: string[];
  timestamp: string;
}

export interface CancelRequest {
  job_ids: string[];
}

export interface CancelResponse {
  cancelled: string[];
  failed: string[];
  errors?: Record<string, string>;
  timestamp: string;
}

export interface DaemonStatus {
  version: string;
  uptime: string;
  total_jobs: number;
  running_jobs: number;
  completed_jobs: number;
  failed_jobs: number;
  cancelled_jobs: number;
  timestamp: string;
}

export interface ScheduledJob {
  id?: string;
  name: string;
  description?: string;
  schedule: string;
  job_template: JobDefinition;
  enabled?: boolean;
  created_at?: string;
  updated_at?: string;
  next_run?: string;
  last_run?: string;
  run_count?: number;
  tags?: string[];
}

export interface Webhook {
  url: string;
  events?: string[];
  headers?: Record<string, string>;
}

export interface ErrorResponse {
  error: string;
  timestamp: string;
}

export interface HealthResponse {
  status: string;
  timestamp: string;
}

export interface CapabilitiesResponse {
  capabilities: any;
  default_method: string;
  timestamp: string;
}

// Carbon-Aware Scheduling Models

export interface CarbonForecast {
  time: string;
  intensity_gco2_kwh: number;
  quality: string;
}

export interface CarbonStatus {
  zone: string;
  current_intensity: number;
  renewable_percent: number;
  optimal_for_backup: boolean;
  next_optimal_time?: string;
  forecast_next_4h: CarbonForecast[];
  reasoning: string;
  quality: string;
  timestamp: string;
}

export interface CarbonReport {
  operation_id: string;
  start_time: string;
  end_time: string;
  duration_hours: number;
  data_size_gb: number;
  energy_kwh: number;
  carbon_intensity_gco2_kwh: number;
  carbon_emissions_kg_co2: number;
  savings_vs_worst_kg_co2: number;
  renewable_percent: number;
  equivalent: string;
}

export interface CarbonZone {
  id: string;
  name: string;
  region: string;
  description: string;
  typical_intensity: number;
}

export interface CarbonEstimate {
  current_intensity_gco2_kwh: number;
  current_emissions_kg_co2: number;
  best_intensity_gco2_kwh: number;
  best_emissions_kg_co2: number;
  best_time?: string;
  savings_kg_co2: number;
  savings_percent: number;
  recommendation: string;
  delay_minutes?: number;
  forecast: CarbonForecast[];
}
