// WebSocket connection
let ws = null;
let reconnectInterval = null;
let charts = {};
let metricsHistory = {
    timestamps: [],
    jobsActive: [],
    jobsCompleted: [],
    jobsFailed: [],
    memoryUsage: [],
    cpuUsage: []
};
const MAX_HISTORY = 50;

// Initialize dashboard
document.addEventListener('DOMContentLoaded', () => {
    initializeCharts();
    connectWebSocket();
    updateTimestamp();
    setInterval(updateTimestamp, 1000);
});

// Connect to WebSocket
function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    try {
        ws = new WebSocket(wsUrl);

        ws.onopen = () => {
            console.log('WebSocket connected');
            updateConnectionStatus(true);
            clearInterval(reconnectInterval);
        };

        ws.onmessage = (event) => {
            try {
                const metrics = JSON.parse(event.data);
                updateDashboard(metrics);
            } catch (error) {
                console.error('Error parsing WebSocket message:', error);
            }
        };

        ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            updateConnectionStatus(false);
        };

        ws.onclose = () => {
            console.log('WebSocket disconnected');
            updateConnectionStatus(false);
            // Attempt to reconnect
            reconnectInterval = setInterval(() => {
                console.log('Attempting to reconnect...');
                connectWebSocket();
            }, 5000);
        };
    } catch (error) {
        console.error('Failed to create WebSocket:', error);
        updateConnectionStatus(false);
    }
}

// Update connection status
function updateConnectionStatus(connected) {
    const statusDot = document.getElementById('statusDot');
    const statusText = document.getElementById('statusText');

    if (connected) {
        statusDot.classList.add('connected');
        statusText.textContent = 'Connected';
    } else {
        statusDot.classList.remove('connected');
        statusText.textContent = 'Disconnected';
    }
}

// Update timestamp
function updateTimestamp() {
    const timestamp = document.getElementById('timestamp');
    const now = new Date();
    timestamp.textContent = now.toLocaleString();
}

// Initialize charts
function initializeCharts() {
    const chartOptions = {
        responsive: true,
        maintainAspectRatio: true,
        plugins: {
            legend: {
                labels: {
                    color: '#f3f4f6'
                }
            }
        },
        scales: {
            y: {
                ticks: {
                    color: '#9ca3af'
                },
                grid: {
                    color: '#4b5563'
                }
            },
            x: {
                ticks: {
                    color: '#9ca3af'
                },
                grid: {
                    color: '#4b5563'
                }
            }
        }
    };

    // Job Status Pie Chart
    charts.jobStatus = new Chart(
        document.getElementById('jobStatusChart'),
        {
            type: 'doughnut',
            data: {
                labels: ['Active', 'Completed', 'Failed', 'Pending'],
                datasets: [{
                    data: [0, 0, 0, 0],
                    backgroundColor: [
                        '#3b82f6',
                        '#10b981',
                        '#ef4444',
                        '#f59e0b'
                    ]
                }]
            },
            options: {
                ...chartOptions,
                plugins: {
                    legend: {
                        position: 'bottom',
                        labels: {
                            color: '#f3f4f6'
                        }
                    }
                }
            }
        }
    );

    // Jobs Over Time Line Chart
    charts.jobsTime = new Chart(
        document.getElementById('jobsTimeChart'),
        {
            type: 'line',
            data: {
                labels: [],
                datasets: [
                    {
                        label: 'Active',
                        data: [],
                        borderColor: '#3b82f6',
                        tension: 0.4
                    },
                    {
                        label: 'Completed',
                        data: [],
                        borderColor: '#10b981',
                        tension: 0.4
                    },
                    {
                        label: 'Failed',
                        data: [],
                        borderColor: '#ef4444',
                        tension: 0.4
                    }
                ]
            },
            options: chartOptions
        }
    );

    // Provider Distribution Bar Chart
    charts.provider = new Chart(
        document.getElementById('providerChart'),
        {
            type: 'bar',
            data: {
                labels: [],
                datasets: [{
                    label: 'Jobs',
                    data: [],
                    backgroundColor: '#3b82f6'
                }]
            },
            options: chartOptions
        }
    );

    // System Resources Chart
    charts.resources = new Chart(
        document.getElementById('resourcesChart'),
        {
            type: 'line',
            data: {
                labels: [],
                datasets: [
                    {
                        label: 'Memory (MB)',
                        data: [],
                        borderColor: '#10b981',
                        yAxisID: 'y',
                        tension: 0.4
                    },
                    {
                        label: 'CPU (%)',
                        data: [],
                        borderColor: '#f59e0b',
                        yAxisID: 'y1',
                        tension: 0.4
                    }
                ]
            },
            options: {
                ...chartOptions,
                scales: {
                    ...chartOptions.scales,
                    y1: {
                        type: 'linear',
                        display: true,
                        position: 'right',
                        ticks: {
                            color: '#9ca3af'
                        },
                        grid: {
                            drawOnChartArea: false
                        }
                    }
                }
            }
        }
    );
}

// Update dashboard with new metrics
function updateDashboard(metrics) {
    // Update stat cards
    document.getElementById('jobsActive').textContent = metrics.jobs_active || 0;
    document.getElementById('jobsCompleted').textContent = metrics.jobs_completed || 0;
    document.getElementById('jobsFailed').textContent = metrics.jobs_failed || 0;
    document.getElementById('queueLength').textContent = metrics.queue_length || 0;
    document.getElementById('httpRequests').textContent = formatNumber(metrics.http_requests || 0);
    document.getElementById('avgResponse').textContent = `${(metrics.avg_response_time || 0).toFixed(1)}ms`;
    document.getElementById('memoryUsage').textContent = `${metrics.memory_usage || 0}MB`;
    document.getElementById('cpuUsage').textContent = `${(metrics.cpu_usage || 0).toFixed(1)}%`;
    document.getElementById('clientCount').textContent = metrics.active_connections || 0;

    // Update system health
    const healthElement = document.getElementById('systemHealth');
    healthElement.textContent = metrics.system_health || 'Unknown';
    healthElement.className = `health-${metrics.system_health || 'unknown'}`;

    // Update metrics history
    updateMetricsHistory(metrics);

    // Update charts
    updateCharts(metrics);

    // Update jobs table
    updateJobsTable(metrics.recent_jobs || []);

    // Update alerts
    updateAlerts(metrics.alerts || []);
}

// Update metrics history
function updateMetricsHistory(metrics) {
    const timestamp = new Date(metrics.timestamp).toLocaleTimeString();

    metricsHistory.timestamps.push(timestamp);
    metricsHistory.jobsActive.push(metrics.jobs_active || 0);
    metricsHistory.jobsCompleted.push(metrics.jobs_completed || 0);
    metricsHistory.jobsFailed.push(metrics.jobs_failed || 0);
    metricsHistory.memoryUsage.push(metrics.memory_usage || 0);
    metricsHistory.cpuUsage.push(metrics.cpu_usage || 0);

    // Keep only last MAX_HISTORY entries
    if (metricsHistory.timestamps.length > MAX_HISTORY) {
        Object.keys(metricsHistory).forEach(key => {
            metricsHistory[key].shift();
        });
    }
}

// Update charts
function updateCharts(metrics) {
    // Job Status Chart
    charts.jobStatus.data.datasets[0].data = [
        metrics.jobs_active || 0,
        metrics.jobs_completed || 0,
        metrics.jobs_failed || 0,
        metrics.jobs_pending || 0
    ];
    charts.jobStatus.update('none');

    // Jobs Over Time Chart
    charts.jobsTime.data.labels = metricsHistory.timestamps;
    charts.jobsTime.data.datasets[0].data = metricsHistory.jobsActive;
    charts.jobsTime.data.datasets[1].data = metricsHistory.jobsCompleted;
    charts.jobsTime.data.datasets[2].data = metricsHistory.jobsFailed;
    charts.jobsTime.update('none');

    // Provider Distribution Chart
    if (metrics.provider_stats) {
        charts.provider.data.labels = Object.keys(metrics.provider_stats);
        charts.provider.data.datasets[0].data = Object.values(metrics.provider_stats);
        charts.provider.update('none');
    }

    // Resources Chart
    charts.resources.data.labels = metricsHistory.timestamps;
    charts.resources.data.datasets[0].data = metricsHistory.memoryUsage;
    charts.resources.data.datasets[1].data = metricsHistory.cpuUsage;
    charts.resources.update('none');
}

// Update jobs table
function updateJobsTable(jobs) {
    const tbody = document.getElementById('jobsTableBody');

    if (!jobs || jobs.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" class="no-data">No jobs yet</td></tr>';
        return;
    }

    tbody.innerHTML = jobs.map(job => `
        <tr>
            <td>${escapeHtml(job.id)}</td>
            <td>${escapeHtml(job.name)}</td>
            <td><span class="status-badge status-${job.status}">${job.status}</span></td>
            <td>
                <div class="progress-bar">
                    <div class="progress-fill" style="width: ${job.progress}%"></div>
                </div>
                <small>${job.progress}%</small>
            </td>
            <td>${escapeHtml(job.provider)}</td>
            <td>${escapeHtml(job.vm_name)}</td>
            <td>${formatDuration(job.duration)}</td>
            <td>${formatTime(job.start_time)}</td>
        </tr>
    `).join('');
}

// Update alerts
function updateAlerts(alerts) {
    const container = document.getElementById('alertsContainer');

    if (!alerts || alerts.length === 0) {
        container.innerHTML = '';
        return;
    }

    // Show only first 3 alerts
    const displayAlerts = alerts.slice(0, 3);

    container.innerHTML = displayAlerts.map(alert => `
        <div class="alert alert-${alert.severity}">
            <div>
                <strong>${alert.severity.toUpperCase()}:</strong> ${escapeHtml(alert.message)}
                <small style="display: block; margin-top: 5px; color: var(--text-secondary);">
                    ${formatTime(alert.time)}
                </small>
            </div>
            <span class="alert-close" onclick="this.parentElement.remove()">✕</span>
        </div>
    `).join('');
}

// Utility functions
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function formatNumber(num) {
    if (num >= 1000000) {
        return (num / 1000000).toFixed(1) + 'M';
    } else if (num >= 1000) {
        return (num / 1000).toFixed(1) + 'K';
    }
    return num.toString();
}

function formatDuration(seconds) {
    if (!seconds) return '0s';

    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = Math.floor(seconds % 60);

    if (hours > 0) {
        return `${hours}h ${minutes}m`;
    } else if (minutes > 0) {
        return `${minutes}m ${secs}s`;
    }
    return `${secs}s`;
}

function formatTime(timeString) {
    if (!timeString) return '-';
    const date = new Date(timeString);
    return date.toLocaleString();
}
