// SPDX-License-Identifier: Apache-2.0

/**
 * HyperSDK TypeScript Client
 *
 * VM migration and export platform client library.
 */

export { HyperSDK, HyperSDKConfig } from './client';

export {
  JobStatus,
  ExportFormat,
  ExportMethod,
  VCenterConfig,
  ExportOptions,
  JobDefinition,
  JobProgress,
  JobResult,
  Job,
  QueryRequest,
  QueryResponse,
  SubmitResponse,
  CancelRequest,
  CancelResponse,
  DaemonStatus,
  ScheduledJob,
  Webhook,
  ErrorResponse,
  HealthResponse,
  CapabilitiesResponse,
  CarbonStatus,
  CarbonForecast,
  CarbonReport,
  CarbonZone,
  CarbonEstimate,
} from './models';

export {
  HyperSDKError,
  AuthenticationError,
  JobNotFoundError,
  APIError,
  ValidationError,
  WebSocketError,
} from './errors';

export default { HyperSDK };
