// SPDX-License-Identifier: Apache-2.0

import React, { useState, useEffect } from 'react';
import type { WorkflowStatus, WorkflowJob } from '../types/metrics';
import { formatDuration } from '../utils/formatters';

interface WorkflowDashboardProps {
  apiUrl?: string;
}

export const WorkflowDashboard: React.FC<WorkflowDashboardProps> = ({ apiUrl = '' }) => {
  const [status, setStatus] = useState<WorkflowStatus | null>(null);
  const [jobs, setJobs] = useState<WorkflowJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(true);

  const fetchWorkflowStatus = async () => {
    try {
      const response = await fetch(`${apiUrl}/api/workflow/status`);
      if (!response.ok) throw new Error('Failed to fetch workflow status');
      const data = await response.json();
      setStatus(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    }
  };

  const fetchActiveJobs = async () => {
    try {
      const response = await fetch(`${apiUrl}/api/workflow/jobs/active`);
      if (!response.ok) throw new Error('Failed to fetch active jobs');
      const data = await response.json();
      setJobs(data.jobs || []);
    } catch (err) {
      console.error('Failed to fetch jobs:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchWorkflowStatus();
    fetchActiveJobs();

    if (autoRefresh) {
      const interval = setInterval(() => {
        fetchWorkflowStatus();
        fetchActiveJobs();
      }, 3000);
      return () => clearInterval(interval);
    }
  }, [autoRefresh, apiUrl]);

  const cardStyle: React.CSSProperties = {
    backgroundColor: '#fff',
    borderRadius: '4px',
    border: '2px solid #e0e0e0',
    padding: '12px',
    marginBottom: '12px',
  };

  const headerStyle: React.CSSProperties = {
    display: 'flex',
    alignItems: 'center',
    gap: '6px',
    marginBottom: '12px',
  };

  const accentBarStyle: React.CSSProperties = {
    width: '2px',
    height: '14px',
    backgroundColor: '#f0583a',
  };

  const statGridStyle: React.CSSProperties = {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))',
    gap: '10px',
  };

  const statBoxStyle: React.CSSProperties = {
    padding: '8px',
    backgroundColor: '#f8f9fa',
    borderRadius: '3px',
    border: '1px solid #e0e0e0',
  };

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '40px', fontSize: '12px', color: '#666' }}>
        Loading workflow status...
      </div>
    );
  }

  if (error) {
    return (
      <div style={{ ...cardStyle, borderColor: '#dc3545', color: '#dc3545' }}>
        <div style={headerStyle}>
          <div style={{ ...accentBarStyle, backgroundColor: '#dc3545' }} />
          <h3 style={{ margin: 0, fontSize: '12px', fontWeight: '600' }}>Workflow Error</h3>
        </div>
        <p style={{ margin: 0, fontSize: '11px' }}>{error}</p>
        <p style={{ margin: '8px 0 0', fontSize: '10px', color: '#666' }}>
          Make sure the workflow daemon is running and accessible.
        </p>
      </div>
    );
  }

  if (!status) return null;

  return (
    <div>
      {/* Header with controls */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
        <h2 style={{ margin: 0, fontSize: '14px', fontWeight: '600' }}>Workflow Daemon</h2>
        <label style={{ fontSize: '10px', display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer' }}>
          <input
            type="checkbox"
            checked={autoRefresh}
            onChange={(e) => setAutoRefresh(e.target.checked)}
          />
          Auto-refresh
        </label>
      </div>

      {/* Status Overview */}
      <div style={cardStyle}>
        <div style={headerStyle}>
          <div style={accentBarStyle} />
          <h3 style={{ margin: 0, fontSize: '12px', fontWeight: '600' }}>Status Overview</h3>
          <span style={{
            marginLeft: 'auto',
            padding: '2px 8px',
            borderRadius: '3px',
            fontSize: '9px',
            fontWeight: '600',
            backgroundColor: status.running ? '#d4edda' : '#f8d7da',
            color: status.running ? '#155724' : '#721c24',
          }}>
            {status.running ? '● Running' : '● Stopped'}
          </span>
        </div>

        <div style={statGridStyle}>
          <div style={statBoxStyle}>
            <div style={{ fontSize: '9px', color: '#666', marginBottom: '4px' }}>Mode</div>
            <div style={{ fontSize: '12px', fontWeight: '600', textTransform: 'uppercase' }}>
              {status.mode}
            </div>
          </div>

          <div style={statBoxStyle}>
            <div style={{ fontSize: '9px', color: '#666', marginBottom: '4px' }}>Queue Depth</div>
            <div style={{ fontSize: '12px', fontWeight: '600', color: '#0066cc' }}>
              {status.queue_depth}
            </div>
          </div>

          <div style={statBoxStyle}>
            <div style={{ fontSize: '9px', color: '#666', marginBottom: '4px' }}>Active Jobs</div>
            <div style={{ fontSize: '12px', fontWeight: '600', color: '#ff9800' }}>
              {status.active_jobs}
            </div>
          </div>

          <div style={statBoxStyle}>
            <div style={{ fontSize: '9px', color: '#666', marginBottom: '4px' }}>Max Workers</div>
            <div style={{ fontSize: '12px', fontWeight: '600' }}>
              {status.max_workers}
            </div>
          </div>

          <div style={statBoxStyle}>
            <div style={{ fontSize: '9px', color: '#666', marginBottom: '4px' }}>Processed (Today)</div>
            <div style={{ fontSize: '12px', fontWeight: '600', color: '#28a745' }}>
              {status.processed_today}
            </div>
          </div>

          <div style={statBoxStyle}>
            <div style={{ fontSize: '9px', color: '#666', marginBottom: '4px' }}>Failed (Today)</div>
            <div style={{ fontSize: '12px', fontWeight: '600', color: '#dc3545' }}>
              {status.failed_today}
            </div>
          </div>

          <div style={statBoxStyle}>
            <div style={{ fontSize: '9px', color: '#666', marginBottom: '4px' }}>Uptime</div>
            <div style={{ fontSize: '12px', fontWeight: '600' }}>
              {formatDuration(status.uptime_seconds)}
            </div>
          </div>
        </div>
      </div>

      {/* Active Jobs */}
      <div style={cardStyle}>
        <div style={headerStyle}>
          <div style={accentBarStyle} />
          <h3 style={{ margin: 0, fontSize: '12px', fontWeight: '600' }}>
            Active Jobs ({jobs.length})
          </h3>
        </div>

        {jobs.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '20px', fontSize: '11px', color: '#666' }}>
            No active jobs
          </div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr>
                  <th style={{
                    padding: '6px 8px',
                    textAlign: 'left',
                    fontSize: '9px',
                    fontWeight: '600',
                    backgroundColor: '#f0f2f7',
                    borderBottom: '1px solid #e0e0e0',
                  }}>
                    Job ID
                  </th>
                  <th style={{
                    padding: '6px 8px',
                    textAlign: 'left',
                    fontSize: '9px',
                    fontWeight: '600',
                    backgroundColor: '#f0f2f7',
                    borderBottom: '1px solid #e0e0e0',
                  }}>
                    Name
                  </th>
                  <th style={{
                    padding: '6px 8px',
                    textAlign: 'left',
                    fontSize: '9px',
                    fontWeight: '600',
                    backgroundColor: '#f0f2f7',
                    borderBottom: '1px solid #e0e0e0',
                  }}>
                    Stage
                  </th>
                  <th style={{
                    padding: '6px 8px',
                    textAlign: 'left',
                    fontSize: '9px',
                    fontWeight: '600',
                    backgroundColor: '#f0f2f7',
                    borderBottom: '1px solid #e0e0e0',
                  }}>
                    Progress
                  </th>
                  <th style={{
                    padding: '6px 8px',
                    textAlign: 'left',
                    fontSize: '9px',
                    fontWeight: '600',
                    backgroundColor: '#f0f2f7',
                    borderBottom: '1px solid #e0e0e0',
                  }}>
                    Elapsed
                  </th>
                </tr>
              </thead>
              <tbody>
                {jobs.map((job) => (
                  <tr key={job.id}>
                    <td style={{ padding: '6px 8px', fontSize: '10px', borderBottom: '1px solid #e0e0e0' }}>
                      <code style={{ fontSize: '9px', backgroundColor: '#f8f9fa', padding: '2px 4px', borderRadius: '2px' }}>
                        {job.id.substring(0, 12)}...
                      </code>
                    </td>
                    <td style={{ padding: '6px 8px', fontSize: '10px', borderBottom: '1px solid #e0e0e0' }}>
                      {job.name}
                    </td>
                    <td style={{ padding: '6px 8px', fontSize: '10px', borderBottom: '1px solid #e0e0e0' }}>
                      <span style={{
                        padding: '2px 6px',
                        borderRadius: '3px',
                        fontSize: '9px',
                        backgroundColor: '#fff3cd',
                        color: '#856404',
                      }}>
                        {job.stage}
                      </span>
                    </td>
                    <td style={{ padding: '6px 8px', fontSize: '10px', borderBottom: '1px solid #e0e0e0' }}>
                      {job.progress > 0 ? (
                        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                          <div style={{
                            flex: 1,
                            height: '6px',
                            backgroundColor: '#e0e0e0',
                            borderRadius: '3px',
                            overflow: 'hidden',
                          }}>
                            <div style={{
                              width: `${job.progress}%`,
                              height: '100%',
                              backgroundColor: '#28a745',
                              transition: 'width 0.3s ease',
                            }} />
                          </div>
                          <span style={{ fontSize: '9px', minWidth: '35px' }}>{job.progress}%</span>
                        </div>
                      ) : (
                        <span style={{ fontSize: '9px', color: '#666' }}>-</span>
                      )}
                    </td>
                    <td style={{ padding: '6px 8px', fontSize: '10px', borderBottom: '1px solid #e0e0e0' }}>
                      {formatDuration(job.elapsed_seconds)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Quick Actions */}
      <div style={cardStyle}>
        <div style={headerStyle}>
          <div style={accentBarStyle} />
          <h3 style={{ margin: 0, fontSize: '12px', fontWeight: '600' }}>Quick Actions</h3>
        </div>

        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
          <button
            onClick={() => {
              fetchWorkflowStatus();
              fetchActiveJobs();
            }}
            style={{
              padding: '6px 12px',
              fontSize: '10px',
              fontWeight: '600',
              borderRadius: '3px',
              border: '1px solid #222324',
              backgroundColor: '#fff',
              cursor: 'pointer',
              transition: 'all 0.2s',
            }}
          >
            Refresh Status
          </button>

          <a
            href="#manifest-builder"
            style={{
              padding: '6px 12px',
              fontSize: '10px',
              fontWeight: '600',
              borderRadius: '3px',
              border: '1px solid #222324',
              backgroundColor: '#f0583a',
              color: '#fff',
              textDecoration: 'none',
              display: 'inline-block',
              transition: 'all 0.2s',
            }}
          >
            Create Manifest
          </a>

          <span style={{ fontSize: '10px', color: '#666', padding: '6px 0', display: 'flex', alignItems: 'center' }}>
            Mode: <strong style={{ marginLeft: '4px', textTransform: 'uppercase' }}>{status.mode}</strong>
          </span>
        </div>
      </div>
    </div>
  );
};
