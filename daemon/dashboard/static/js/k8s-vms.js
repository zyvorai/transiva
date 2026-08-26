// SPDX-License-Identifier: Apache-2.0

class VMDashboard {
    constructor() {
        this.refreshInterval = 5000; // 5 seconds
        this.refreshTimer = null;
        this.ws = null;
        this.currentTab = 'running';
        this.data = {
            running_vms: [],
            stopped_vms: [],
            templates: [],
            recent_snapshots: [],
            virtual_machines: {},
            vm_resource_stats: {}
        };
    }

    init() {
        this.setupTabs();
        this.setupWebSocket();
        this.loadData();
        this.startAutoRefresh();
    }

    setupTabs() {
        const tabs = document.querySelectorAll('.k8s-tab');
        tabs.forEach(tab => {
            tab.addEventListener('click', () => {
                this.switchTab(tab.dataset.tab);
            });
        });
    }

    switchTab(tabName) {
        this.currentTab = tabName;

        // Update tab buttons
        document.querySelectorAll('.k8s-tab').forEach(tab => {
            tab.classList.remove('active');
            if (tab.dataset.tab === tabName) {
                tab.classList.add('active');
            }
        });

        // Update tab content
        document.querySelectorAll('.tab-content').forEach(content => {
            content.classList.remove('active');
        });
        document.getElementById(`tab-${tabName}`).classList.add('active');
    }

    setupWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws/k8s`;

        try {
            this.ws = new WebSocket(wsUrl);

            this.ws.onopen = () => {
                console.log('WebSocket connected');
            };

            this.ws.onmessage = (event) => {
                const data = JSON.parse(event.data);
                this.handleWebSocketData(data);
            };

            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error);
            };

            this.ws.onclose = () => {
                console.log('WebSocket disconnected, reconnecting...');
                setTimeout(() => this.setupWebSocket(), 5000);
            };
        } catch (error) {
            console.error('Failed to setup WebSocket:', error);
        }
    }

    handleWebSocketData(data) {
        if (data.running_vms) {
            this.data = data;
            this.render();
        }
    }

    async loadData() {
        try {
            const response = await fetch('/api/k8s/metrics');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            this.data = await response.json();
            this.render();
        } catch (error) {
            console.error('Failed to load VM data:', error);
            this.showError('Failed to load VM data');
        }
    }

    render() {
        this.renderStats();
        this.renderRunningVMs();
        this.renderStoppedVMs();
        this.renderTemplates();
        this.renderSnapshots();
        this.updateBadges();
    }

    renderStats() {
        const vms = this.data.virtual_machines || {};
        const resources = this.data.vm_resource_stats || {};

        document.getElementById('stat-total').textContent = vms.total || 0;
        document.getElementById('stat-running').textContent = vms.running || 0;
        document.getElementById('stat-stopped').textContent = vms.stopped || 0;
        document.getElementById('stat-cpus').textContent = resources.total_cpus || 0;
        document.getElementById('stat-memory').textContent =
            (resources.total_memory_gi || 0).toFixed(1);
    }

    renderRunningVMs() {
        const container = document.getElementById('running-vms-list');
        const vms = this.data.running_vms || [];

        if (vms.length === 0) {
            container.innerHTML = this.renderEmptyState(
                '🖥️',
                'No Running VMs',
                'No virtual machines are currently running.'
            );
            return;
        }

        container.innerHTML = vms.map(vm => this.renderVMCard(vm)).join('');
    }

    renderStoppedVMs() {
        const container = document.getElementById('stopped-vms-list');
        const vms = this.data.stopped_vms || [];

        if (vms.length === 0) {
            container.innerHTML = this.renderEmptyState(
                '⏸️',
                'No Stopped VMs',
                'No virtual machines are currently stopped.'
            );
            return;
        }

        container.innerHTML = vms.map(vm => this.renderVMCard(vm)).join('');
    }

    renderTemplates() {
        const container = document.getElementById('templates-list');
        const templates = this.data.templates || [];

        if (templates.length === 0) {
            container.innerHTML = this.renderEmptyState(
                '📋',
                'No VM Templates',
                'No VM templates have been created yet.'
            );
            return;
        }

        container.innerHTML = templates.map(template => this.renderTemplateCard(template)).join('');
    }

    renderSnapshots() {
        const container = document.getElementById('snapshots-list');
        const snapshots = this.data.recent_snapshots || [];

        if (snapshots.length === 0) {
            container.innerHTML = this.renderEmptyState(
                '📸',
                'No VM Snapshots',
                'No VM snapshots have been created yet.'
            );
            return;
        }

        container.innerHTML = snapshots.map(snapshot => this.renderSnapshotCard(snapshot)).join('');
    }

    renderVMCard(vm) {
        const phase = vm.phase || 'Unknown';
        const ipAddrs = (vm.ip_addresses || []).join(', ') || 'N/A';
        const cpuUsage = vm.cpu_usage || 'N/A';
        const memoryUsage = vm.memory_usage || 'N/A';
        const uptime = vm.start_time ? this.formatRelativeTime(vm.start_time) : 'N/A';

        return `
            <div class="vm-card ${phase.toLowerCase()}">
                <div class="vm-header">
                    <div>
                        <div class="vm-name">${this.escapeHtml(vm.name)}</div>
                        <div style="font-size: 0.875rem; color: var(--text-secondary); margin-top: 0.25rem;">
                            ${this.escapeHtml(vm.namespace)}
                        </div>
                    </div>
                    <span class="vm-status status-${phase}">${phase}</span>
                </div>

                <div class="vm-details">
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">vCPUs</div>
                        <div class="vm-detail-value">${vm.cpus} cores</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Memory</div>
                        <div class="vm-detail-value">${vm.memory}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Node</div>
                        <div class="vm-detail-value">${vm.node_name || 'N/A'}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">IP Addresses</div>
                        <div class="vm-detail-value">${ipAddrs}</div>
                    </div>
                    ${phase === 'Running' ? `
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">CPU Usage</div>
                        <div class="vm-detail-value">${cpuUsage}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Memory Usage</div>
                        <div class="vm-detail-value">${memoryUsage}</div>
                    </div>
                    ` : ''}
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Disks</div>
                        <div class="vm-detail-value">${vm.disk_count || 0}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Networks</div>
                        <div class="vm-detail-value">${vm.network_count || 0}</div>
                    </div>
                    ${phase === 'Running' ? `
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Uptime</div>
                        <div class="vm-detail-value">${uptime}</div>
                    </div>
                    ` : ''}
                    ${vm.carbon_intensity ? `
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Carbon Intensity</div>
                        <div class="vm-detail-value">${vm.carbon_intensity.toFixed(0)} gCO₂/kWh</div>
                    </div>
                    ` : ''}
                </div>

                <div class="vm-actions">
                    ${this.renderVMActions(vm)}
                </div>
            </div>
        `;
    }

    renderVMActions(vm) {
        const phase = vm.phase || 'Unknown';
        let actions = [];

        if (phase === 'Running') {
            actions.push(`<button class="vm-action-btn secondary" onclick="vmDashboard.stopVM('${vm.namespace}', '${vm.name}')">⏸️ Stop</button>`);
            actions.push(`<button class="vm-action-btn secondary" onclick="vmDashboard.restartVM('${vm.namespace}', '${vm.name}')">🔄 Restart</button>`);
        } else if (phase === 'Stopped') {
            actions.push(`<button class="vm-action-btn primary" onclick="vmDashboard.startVM('${vm.namespace}', '${vm.name}')">▶️ Start</button>`);
        }

        actions.push(`<button class="vm-action-btn secondary" onclick="vmDashboard.cloneVM('${vm.namespace}', '${vm.name}')">📋 Clone</button>`);
        actions.push(`<button class="vm-action-btn secondary" onclick="vmDashboard.snapshotVM('${vm.namespace}', '${vm.name}')">📸 Snapshot</button>`);

        if (phase === 'Running') {
            actions.push(`<button class="vm-action-btn secondary" onclick="vmDashboard.migrateVM('${vm.namespace}', '${vm.name}')">🔀 Migrate</button>`);
        }

        actions.push(`<button class="vm-action-btn danger" onclick="vmDashboard.deleteVM('${vm.namespace}', '${vm.name}')">🗑️ Delete</button>`);

        return actions.join('');
    }

    renderTemplateCard(template) {
        const tags = (template.tags || []).join(', ') || 'None';
        const osInfo = template.os_type ? `${template.os_type} ${template.os_version || ''}` : 'N/A';

        return `
            <div class="vm-card">
                <div class="vm-header">
                    <div>
                        <div class="vm-name">${this.escapeHtml(template.display_name || template.name)}</div>
                        <div style="font-size: 0.875rem; color: var(--text-secondary); margin-top: 0.25rem;">
                            ${this.escapeHtml(template.description || 'No description')}
                        </div>
                    </div>
                    <span class="vm-status ${template.ready ? 'status-Running' : 'status-Stopped'}">
                        ${template.ready ? 'Ready' : 'Not Ready'}
                    </span>
                </div>

                <div class="vm-details">
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">OS</div>
                        <div class="vm-detail-value">${osInfo}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Version</div>
                        <div class="vm-detail-value">${template.version || 'N/A'}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Default vCPUs</div>
                        <div class="vm-detail-value">${template.default_cpus || 'N/A'}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Default Memory</div>
                        <div class="vm-detail-value">${template.default_memory || 'N/A'}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Usage Count</div>
                        <div class="vm-detail-value">${template.usage_count || 0}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Tags</div>
                        <div class="vm-detail-value">${tags}</div>
                    </div>
                </div>

                <div class="vm-actions">
                    <button class="vm-action-btn primary" onclick="vmDashboard.createVMFromTemplate('${template.namespace}', '${template.name}')">
                        🚀 Create VM
                    </button>
                </div>
            </div>
        `;
    }

    renderSnapshotCard(snapshot) {
        const createdAt = snapshot.creation_time ? this.formatRelativeTime(snapshot.creation_time) : 'N/A';
        const size = snapshot.size || 'Unknown';

        return `
            <div class="vm-card ${snapshot.phase.toLowerCase()}">
                <div class="vm-header">
                    <div>
                        <div class="vm-name">${this.escapeHtml(snapshot.name)}</div>
                        <div style="font-size: 0.875rem; color: var(--text-secondary); margin-top: 0.25rem;">
                            VM: ${this.escapeHtml(snapshot.vm_name)}
                        </div>
                    </div>
                    <span class="vm-status status-${snapshot.phase}">${snapshot.phase}</span>
                </div>

                <div class="vm-details">
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Namespace</div>
                        <div class="vm-detail-value">${snapshot.namespace}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Size</div>
                        <div class="vm-detail-value">${size}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Created</div>
                        <div class="vm-detail-value">${createdAt}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Memory Included</div>
                        <div class="vm-detail-value">${snapshot.include_memory ? 'Yes' : 'No'}</div>
                    </div>
                    <div class="vm-detail-item">
                        <div class="vm-detail-label">Ready to Restore</div>
                        <div class="vm-detail-value">${snapshot.ready_to_restore ? 'Yes' : 'No'}</div>
                    </div>
                </div>

                ${snapshot.description ? `
                <div style="margin-top: 1rem; padding-top: 1rem; border-top: 1px solid var(--bg-tertiary);">
                    <div class="vm-detail-label">Description</div>
                    <div style="color: var(--text-primary); margin-top: 0.5rem;">
                        ${this.escapeHtml(snapshot.description)}
                    </div>
                </div>
                ` : ''}

                <div class="vm-actions" style="margin-top: 1rem;">
                    ${snapshot.ready_to_restore ? `
                    <button class="vm-action-btn primary" onclick="vmDashboard.restoreSnapshot('${snapshot.namespace}', '${snapshot.name}')">
                        ↩️ Restore
                    </button>
                    ` : ''}
                    <button class="vm-action-btn danger" onclick="vmDashboard.deleteSnapshot('${snapshot.namespace}', '${snapshot.name}')">
                        🗑️ Delete
                    </button>
                </div>
            </div>
        `;
    }

    renderEmptyState(icon, title, message) {
        return `
            <div class="empty-state">
                <div class="empty-state-icon">${icon}</div>
                <h3>${title}</h3>
                <p>${message}</p>
            </div>
        `;
    }

    updateBadges() {
        const vms = this.data.virtual_machines || {};
        const templates = this.data.templates || [];
        const snapshots = this.data.recent_snapshots || [];

        document.getElementById('badge-running').textContent = vms.running || 0;
        document.getElementById('badge-stopped').textContent = vms.stopped || 0;
        document.getElementById('badge-templates').textContent = templates.length;
        document.getElementById('badge-snapshots').textContent = snapshots.length;
    }

    // VM Actions
    async startVM(namespace, name) {
        console.log(`Starting VM: ${namespace}/${name}`);
        alert(`Starting VM ${name}. Use kubectl apply with a VMOperation manifest.`);
    }

    async stopVM(namespace, name) {
        console.log(`Stopping VM: ${namespace}/${name}`);
        alert(`Stopping VM ${name}. Use kubectl apply with a VMOperation manifest.`);
    }

    async restartVM(namespace, name) {
        console.log(`Restarting VM: ${namespace}/${name}`);
        alert(`Restarting VM ${name}. Use kubectl apply with a VMOperation manifest.`);
    }

    async cloneVM(namespace, name) {
        const targetName = prompt(`Enter name for cloned VM:`, `${name}-clone`);
        if (!targetName) return;
        console.log(`Cloning VM: ${namespace}/${name} to ${targetName}`);
        alert(`Cloning VM ${name} to ${targetName}. Use hyperctl k8s -op vm-clone.`);
    }

    async snapshotVM(namespace, name) {
        const snapshotName = prompt(`Enter snapshot name:`, `${name}-snapshot-${Date.now()}`);
        if (!snapshotName) return;
        console.log(`Creating snapshot of VM: ${namespace}/${name}`);
        alert(`Creating snapshot ${snapshotName}. Use hyperctl k8s -op vm-snapshot-create.`);
    }

    async migrateVM(namespace, name) {
        const targetNode = prompt(`Enter target node name:`);
        if (!targetNode) return;
        console.log(`Migrating VM: ${namespace}/${name} to ${targetNode}`);
        alert(`Migrating VM ${name} to ${targetNode}. Use hyperctl k8s -op vm-migrate.`);
    }

    async deleteVM(namespace, name) {
        if (!confirm(`Are you sure you want to delete VM ${name}?`)) return;
        console.log(`Deleting VM: ${namespace}/${name}`);
        alert(`Deleting VM ${name}. Use kubectl delete vm ${name}.`);
    }

    async createVMFromTemplate(namespace, templateName) {
        const vmName = prompt(`Enter name for new VM:`);
        if (!vmName) return;
        console.log(`Creating VM from template: ${templateName}`);
        alert(`Creating VM ${vmName} from template ${templateName}. Use hyperctl k8s -op vm-create.`);
    }

    async restoreSnapshot(namespace, name) {
        console.log(`Restoring snapshot: ${namespace}/${name}`);
        alert(`Restoring snapshot ${name}. Use the RestoreJob CRD or hyperctl command.`);
    }

    async deleteSnapshot(namespace, name) {
        if (!confirm(`Are you sure you want to delete snapshot ${name}?`)) return;
        console.log(`Deleting snapshot: ${namespace}/${name}`);
        alert(`Deleting snapshot ${name}. Use kubectl delete vmsnapshot ${name}.`);
    }

    // Utility functions
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    formatRelativeTime(timestamp) {
        const date = new Date(timestamp);
        const now = new Date();
        const seconds = Math.floor((now - date) / 1000);

        if (seconds < 60) return `${seconds}s ago`;
        if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
        if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
        return `${Math.floor(seconds / 86400)}d ago`;
    }

    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    showError(message) {
        console.error(message);
    }

    startAutoRefresh() {
        this.refreshTimer = setInterval(() => {
            this.loadData();
        }, this.refreshInterval);
    }

    stopAutoRefresh() {
        if (this.refreshTimer) {
            clearInterval(this.refreshTimer);
            this.refreshTimer = null;
        }
    }
}
