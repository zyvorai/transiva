// Copyright 2026 Zyvor AI Labs
// SPDX-License-Identifier: Apache-2.0

// HyperSDK Kubernetes Dashboard JavaScript

// Store original data for filtering
let allBackupJobs = [];

// Tab switching
document.querySelectorAll('.nav-tab').forEach(tab => {
    tab.addEventListener('click', () => {
        // Remove active class from all tabs and contents
        document.querySelectorAll('.nav-tab').forEach(t => t.classList.remove('active'));
        document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

        // Add active class to clicked tab and corresponding content
        tab.classList.add('active');
        const tabId = tab.getAttribute('data-tab') + '-tab';
        document.getElementById(tabId).classList.add('active');
    });
});

// Update cluster info
function updateClusterInfo(clusterInfo) {
    const statusEl = document.getElementById('cluster-status');
    const statusIndicator = statusEl.querySelector('.status-indicator');

    if (clusterInfo.connected) {
        statusIndicator.className = 'status-indicator status-running';
        statusEl.innerHTML = '<span class="status-indicator status-running"></span>Connected';

        document.getElementById('cluster-version').textContent = clusterInfo.version || 'Unknown';
        document.getElementById('node-count').textContent = clusterInfo.node_count || 0;
        document.getElementById('namespace-count').textContent = clusterInfo.namespace_count || 0;
        document.getElementById('kubevirt-status').textContent = clusterInfo.kubevirt_enabled ? '✅ Enabled' : '❌ Not detected';
    } else {
        statusIndicator.className = 'status-indicator status-failed';
        statusEl.innerHTML = '<span class="status-indicator status-failed"></span>Disconnected';
    }
}

// Update operator status
function updateOperatorStatus(operatorStatus, operatorReplicas) {
    const statusEl = document.getElementById('operator-status');

    if (operatorStatus === 'Running') {
        statusEl.innerHTML = `<span class="badge badge-success">${operatorStatus}</span> (${operatorReplicas} replicas)`;
    } else if (operatorStatus === 'Degraded') {
        statusEl.innerHTML = `<span class="badge badge-warning">${operatorStatus}</span> (${operatorReplicas} replicas)`;
    } else {
        statusEl.innerHTML = `<span class="badge badge-danger">${operatorStatus}</span>`;
    }
}

// Update overview stats
function updateOverviewStats(metrics) {
    document.getElementById('total-backups').textContent = metrics.backup_jobs.total || 0;
    document.getElementById('running-backups').textContent = metrics.backup_jobs.running || 0;
    document.getElementById('active-schedules').textContent = metrics.backup_schedules.total || 0;
    document.getElementById('pending-restores').textContent = metrics.restore_jobs.pending || 0;
}

// Update backup jobs table
function updateBackupJobsTable(backups) {
    // Store original data for filtering
    allBackupJobs = backups || [];

    // Apply filters
    const filtered = applyBackupFilters();

    const tbody = document.getElementById('backupjobs-tbody');

    if (!filtered || filtered.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="8">
                    <div class="empty-state">
                        <div class="icon">📦</div>
                        <p>No backup jobs found</p>
                        <p style="font-size: 0.875rem; margin-top: 10px;">
                            Create one with: <code>hyperctl k8s -op backup-create ...</code>
                        </p>
                    </div>
                </td>
            </tr>
        `;
        return;
    }

    const backupsToShow = filtered;

    tbody.innerHTML = backupsToShow.map(backup => {
        const phaseBadge = getPhaseBadge(backup.phase);
        const carbonIcon = backup.carbon_aware ? '🌱' : '';
        const size = formatBytes(backup.size);
        const duration = formatDuration(backup.duration);

        return `
            <tr>
                <td><strong>${backup.name}</strong><br><small style="color: #64748b;">${backup.namespace}</small></td>
                <td>${backup.vm_name}</td>
                <td><span class="badge badge-secondary">${backup.provider}</span></td>
                <td>${phaseBadge}</td>
                <td>
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: ${backup.progress}%"></div>
                    </div>
                    <small style="color: #64748b;">${backup.progress}%</small>
                </td>
                <td>${size}</td>
                <td>${carbonIcon}${backup.carbon_intensity ? backup.carbon_intensity.toFixed(1) + ' gCO₂/kWh' : '-'}</td>
                <td>${duration}</td>
            </tr>
        `;
    }).join('');
}

// Update schedules table
function updateSchedulesTable(schedules) {
    const tbody = document.getElementById('schedules-tbody');

    if (!schedules || schedules.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="8">
                    <div class="empty-state">
                        <div class="icon">⏰</div>
                        <p>No backup schedules found</p>
                        <p style="font-size: 0.875rem; margin-top: 10px;">
                            Create one with: <code>hyperctl k8s -op schedule-create ...</code>
                        </p>
                    </div>
                </td>
            </tr>
        `;
        return;
    }

    tbody.innerHTML = schedules.map(schedule => {
        const statusBadge = schedule.suspended ?
            '<span class="badge badge-warning">Suspended</span>' :
            '<span class="badge badge-success">Active</span>';
        const nextRun = schedule.next_schedule_time ?
            formatRelativeTime(new Date(schedule.next_schedule_time)) : '-';

        return `
            <tr>
                <td><strong>${schedule.name}</strong><br><small style="color: #64748b;">${schedule.namespace}</small></td>
                <td><code>${schedule.schedule}</code><br><small style="color: #64748b;">${schedule.timezone}</small></td>
                <td>${schedule.vm_name}</td>
                <td><span class="badge badge-secondary">${schedule.provider}</span></td>
                <td>${statusBadge}</td>
                <td><span style="color: #10b981;">${schedule.successful_jobs || 0}</span></td>
                <td><span style="color: #ef4444;">${schedule.failed_jobs || 0}</span></td>
                <td>${nextRun}</td>
            </tr>
        `;
    }).join('');
}

// Update restores table
function updateRestoresTable(restores) {
    const tbody = document.getElementById('restores-tbody');

    if (!restores || restores.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="8">
                    <div class="empty-state">
                        <div class="icon">♻️</div>
                        <p>No restore jobs found</p>
                        <p style="font-size: 0.875rem; margin-top: 10px;">
                            Create one with: <code>hyperctl k8s -op restore-create ...</code>
                        </p>
                    </div>
                </td>
            </tr>
        `;
        return;
    }

    tbody.innerHTML = restores.map(restore => {
        const phaseBadge = getPhaseBadge(restore.phase);
        const powerOnIcon = restore.power_on ? '✅' : '❌';
        const duration = formatDuration(restore.duration);

        return `
            <tr>
                <td><strong>${restore.name}</strong><br><small style="color: #64748b;">${restore.namespace}</small></td>
                <td>${restore.vm_name}</td>
                <td><span class="badge badge-secondary">${restore.provider}</span></td>
                <td>${restore.source_backup}</td>
                <td>${phaseBadge}</td>
                <td>
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: ${restore.progress}%"></div>
                    </div>
                    <small style="color: #64748b;">${restore.progress}%</small>
                </td>
                <td>${powerOnIcon}</td>
                <td>${duration}</td>
            </tr>
        `;
    }).join('');
}

// Update carbon stats
function updateCarbonStats(carbonStats) {
    document.getElementById('carbon-backups').textContent = carbonStats.carbon_aware_backups || 0;
    document.getElementById('avg-intensity').textContent = (carbonStats.avg_intensity || 0).toFixed(1);
    document.getElementById('carbon-savings').textContent = (carbonStats.estimated_savings_kg || 0).toFixed(2);
    document.getElementById('delayed-backups').textContent = carbonStats.delayed_backups || 0;
}

// Helper functions
function getPhaseBadge(phase) {
    const badges = {
        'Pending': '<span class="badge badge-warning">Pending</span>',
        'Running': '<span class="badge badge-primary">Running</span>',
        'Completed': '<span class="badge badge-success">Completed</span>',
        'Failed': '<span class="badge badge-danger">Failed</span>',
        'Cancelled': '<span class="badge badge-secondary">Cancelled</span>'
    };
    return badges[phase] || `<span class="badge badge-secondary">${phase}</span>`;
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    if (!bytes) return '-';

    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatDuration(seconds) {
    if (!seconds || seconds === 0) return '-';

    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = Math.floor(seconds % 60);

    if (hours > 0) {
        return `${hours}h ${minutes}m`;
    } else if (minutes > 0) {
        return `${minutes}m ${secs}s`;
    } else {
        return `${secs}s`;
    }
}

function formatRelativeTime(date) {
    const now = new Date();
    const diff = date - now;

    if (diff < 0) return 'Past due';

    const hours = Math.floor(diff / (1000 * 60 * 60));
    const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));

    if (hours > 24) {
        const days = Math.floor(hours / 24);
        return `in ${days}d ${hours % 24}h`;
    } else if (hours > 0) {
        return `in ${hours}h ${minutes}m`;
    } else {
        return `in ${minutes}m`;
    }
}

function updateLastUpdateTime() {
    const now = new Date();
    document.getElementById('last-update').textContent =
        `Last updated: ${now.toLocaleTimeString()}`;
}

// Fetch and update all data
async function fetchAndUpdate() {
    try {
        const response = await fetch('/api/k8s/metrics');
        const metrics = await response.json();

        // Update all sections
        updateClusterInfo(metrics.cluster_info);
        updateOperatorStatus(metrics.operator_status, metrics.operator_replicas);
        updateOverviewStats(metrics);
        updateBackupJobsTable(metrics.recent_backups);
        updateSchedulesTable(metrics.active_schedules);
        updateRestoresTable(metrics.recent_restores);
        updateCarbonStats(metrics.carbon_stats);
        updateLastUpdateTime();
    } catch (error) {
        console.error('Failed to fetch metrics:', error);
    }
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    // Initial fetch
    fetchAndUpdate();

    // Update every 5 seconds
    setInterval(fetchAndUpdate, 5000);
});

// WebSocket support for real-time updates (optional)
function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws/k8s`);

    ws.onmessage = (event) => {
        const metrics = JSON.parse(event.data);
        updateClusterInfo(metrics.cluster_info);
        updateOperatorStatus(metrics.operator_status, metrics.operator_replicas);
        updateOverviewStats(metrics);
        updateBackupJobsTable(metrics.recent_backups);
        updateSchedulesTable(metrics.active_schedules);
        updateRestoresTable(metrics.recent_restores);
        updateCarbonStats(metrics.carbon_stats);
        updateLastUpdateTime();
    };

    ws.onclose = () => {
        // Reconnect after 5 seconds
        setTimeout(connectWebSocket, 5000);
    };

    return ws;
}

// Enable WebSocket updates for real-time data
connectWebSocket();

// Filtering functionality
function applyBackupFilters() {
    const searchTerm = document.getElementById('backup-search')?.value.toLowerCase() || '';
    const namespaceFilter = document.getElementById('backup-namespace-filter')?.value || '';
    const providerFilter = document.getElementById('backup-provider-filter')?.value || '';
    const phaseFilter = document.getElementById('backup-phase-filter')?.value || '';

    let filtered = allBackupJobs;

    // Apply search filter
    if (searchTerm) {
        filtered = filtered.filter(backup =>
            backup.name.toLowerCase().includes(searchTerm) ||
            backup.vm_name.toLowerCase().includes(searchTerm)
        );
    }

    // Apply namespace filter
    if (namespaceFilter) {
        filtered = filtered.filter(backup => backup.namespace === namespaceFilter);
    }

    // Apply provider filter
    if (providerFilter) {
        filtered = filtered.filter(backup => backup.provider === providerFilter);
    }

    // Apply phase filter
    if (phaseFilter) {
        filtered = filtered.filter(backup => backup.phase.toLowerCase() === phaseFilter.toLowerCase());
    }

    // Update filter count
    const filterCountEl = document.getElementById('filter-count');
    if (filterCountEl) {
        const total = allBackupJobs.length;
        const shown = filtered.length;
        if (shown === total) {
            filterCountEl.textContent = `Showing all ${total} backup${total !== 1 ? 's' : ''}`;
        } else {
            filterCountEl.textContent = `Showing ${shown} of ${total} backup${total !== 1 ? 's' : ''}`;
        }
    }

    // Populate namespace filter options dynamically
    populateNamespaceFilter();

    return filtered;
}

function populateNamespaceFilter() {
    const namespaceFilter = document.getElementById('backup-namespace-filter');
    if (!namespaceFilter || allBackupJobs.length === 0) return;

    // Get unique namespaces
    const namespaces = [...new Set(allBackupJobs.map(b => b.namespace))].sort();

    // Only update if different
    const currentOptions = Array.from(namespaceFilter.options).slice(1).map(o => o.value);
    const namespacesChanged = JSON.stringify(currentOptions) !== JSON.stringify(namespaces);

    if (namespacesChanged) {
        const currentValue = namespaceFilter.value;
        namespaceFilter.innerHTML = '<option value="">All Namespaces</option>';
        namespaces.forEach(ns => {
            const option = document.createElement('option');
            option.value = ns;
            option.textContent = ns;
            namespaceFilter.appendChild(option);
        });
        namespaceFilter.value = currentValue;
    }
}

// Setup filter event listeners
document.addEventListener('DOMContentLoaded', () => {
    const searchInput = document.getElementById('backup-search');
    const namespaceFilter = document.getElementById('backup-namespace-filter');
    const providerFilter = document.getElementById('backup-provider-filter');
    const phaseFilter = document.getElementById('backup-phase-filter');
    const clearFiltersBtn = document.getElementById('clear-filters');

    if (searchInput) {
        searchInput.addEventListener('input', () => {
            updateBackupJobsTable(allBackupJobs);
        });
    }

    if (namespaceFilter) {
        namespaceFilter.addEventListener('change', () => {
            updateBackupJobsTable(allBackupJobs);
        });
    }

    if (providerFilter) {
        providerFilter.addEventListener('change', () => {
            updateBackupJobsTable(allBackupJobs);
        });
    }

    if (phaseFilter) {
        phaseFilter.addEventListener('change', () => {
            updateBackupJobsTable(allBackupJobs);
        });
    }

    if (clearFiltersBtn) {
        clearFiltersBtn.addEventListener('click', () => {
            if (searchInput) searchInput.value = '';
            if (namespaceFilter) namespaceFilter.value = '';
            if (providerFilter) providerFilter.value = '';
            if (phaseFilter) phaseFilter.value = '';
            updateBackupJobsTable(allBackupJobs);
        });
    }

    // Export buttons
    const exportCsvBtn = document.getElementById('export-csv');
    const exportJsonBtn = document.getElementById('export-json');

    if (exportCsvBtn) {
        exportCsvBtn.addEventListener('click', () => {
            exportToCSV();
        });
    }

    if (exportJsonBtn) {
        exportJsonBtn.addEventListener('click', () => {
            exportToJSON();
        });
    }
});

// Export functions
function exportToCSV() {
    const filtered = applyBackupFilters();

    if (!filtered || filtered.length === 0) {
        alert('No backup jobs to export');
        return;
    }

    // CSV header
    const headers = ['Name', 'Namespace', 'VM Name', 'Provider', 'Phase', 'Progress (%)', 'Size (Bytes)', 'Carbon Aware', 'Carbon Intensity (gCO2/kWh)', 'Duration (seconds)', 'Start Time', 'Completion Time'];

    // CSV rows
    const rows = filtered.map(backup => [
        backup.name,
        backup.namespace,
        backup.vm_name,
        backup.provider,
        backup.phase,
        backup.progress,
        backup.size || 0,
        backup.carbon_aware ? 'Yes' : 'No',
        backup.carbon_intensity || 0,
        backup.duration || 0,
        backup.start_time || '',
        backup.completion_time || ''
    ]);

    // Convert to CSV format
    const csvContent = [
        headers.join(','),
        ...rows.map(row => row.map(cell => {
            // Escape cells containing commas or quotes
            const cellStr = String(cell);
            if (cellStr.includes(',') || cellStr.includes('"') || cellStr.includes('\n')) {
                return '"' + cellStr.replace(/"/g, '""') + '"';
            }
            return cellStr;
        }).join(','))
    ].join('\n');

    // Download CSV
    downloadFile(csvContent, 'transiva-backups-' + new Date().toISOString().split('T')[0] + '.csv', 'text/csv');
}

function exportToJSON() {
    const filtered = applyBackupFilters();

    if (!filtered || filtered.length === 0) {
        alert('No backup jobs to export');
        return;
    }

    // Create export object with metadata
    const exportData = {
        metadata: {
            exported_at: new Date().toISOString(),
            total_count: filtered.length,
            filters_applied: {
                search: document.getElementById('backup-search')?.value || '',
                namespace: document.getElementById('backup-namespace-filter')?.value || '',
                provider: document.getElementById('backup-provider-filter')?.value || '',
                phase: document.getElementById('backup-phase-filter')?.value || ''
            }
        },
        backups: filtered.map(backup => ({
            name: backup.name,
            namespace: backup.namespace,
            vm_name: backup.vm_name,
            provider: backup.provider,
            phase: backup.phase,
            progress: backup.progress,
            size: backup.size || 0,
            size_formatted: formatBytes(backup.size),
            carbon_aware: backup.carbon_aware,
            carbon_intensity: backup.carbon_intensity || 0,
            duration: backup.duration || 0,
            duration_formatted: formatDuration(backup.duration),
            start_time: backup.start_time || null,
            completion_time: backup.completion_time || null,
            destination: backup.destination || {},
            retention: backup.retention || {}
        }))
    };

    // Convert to JSON with pretty formatting
    const jsonContent = JSON.stringify(exportData, null, 2);

    // Download JSON
    downloadFile(jsonContent, 'transiva-backups-' + new Date().toISOString().split('T')[0] + '.json', 'application/json');
}

function downloadFile(content, filename, mimeType) {
    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
}
