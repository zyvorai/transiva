// SPDX-License-Identifier: Apache-2.0

import React, { useState, useEffect } from 'react';

interface ProviderConfig {
  provider: string;
  name: string;
  enabled: boolean;
  connected: boolean;
  config: { [key: string]: string };
  lastChecked?: string;
  error?: string;
}

interface ProviderManagerProps {
  onProviderSelect?: (provider: string) => void;
}

const ProviderManager: React.FC<ProviderManagerProps> = ({ onProviderSelect }) => {
  const [providers, setProviders] = useState<ProviderConfig[]>([]);
  const [showAddModal, setShowAddModal] = useState(false);
  const [selectedProvider, setSelectedProvider] = useState<string>('');
  const [providerConfig, setProviderConfig] = useState<{ [key: string]: string }>({});
  const [testing, setTesting] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const AVAILABLE_PROVIDERS = [
    { id: 'vsphere', name: 'VMware vSphere', icon: '☁️' },
    { id: 'aws', name: 'Amazon AWS EC2', icon: '🌐' },
    { id: 'azure', name: 'Microsoft Azure', icon: '🔷' },
    { id: 'gcp', name: 'Google Cloud Platform', icon: '🔶' },
    { id: 'hyperv', name: 'Microsoft Hyper-V', icon: '💻' },
    { id: 'oci', name: 'Oracle Cloud Infrastructure', icon: '🟧' },
    { id: 'openstack', name: 'OpenStack', icon: '🌀' },
    { id: 'alibabacloud', name: 'Alibaba Cloud', icon: '🟠' },
    { id: 'proxmox', name: 'Proxmox VE', icon: '🔧' },
  ];

  useEffect(() => {
    loadProviders();
  }, []);

  const loadProviders = async () => {
    try {
      const response = await fetch('/api/providers/list');
      if (response.ok) {
        const data = await response.json();
        setProviders(data);
      }
    } catch (err) {
      console.error('Failed to load providers:', err);
    }
  };

  const testConnection = async (providerId: string) => {
    setTesting(providerId);
    try {
      const response = await fetch('/api/providers/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ provider: providerId }),
      });

      const result = await response.json();

      setProviders(prev => prev.map(p =>
        p.provider === providerId
          ? { ...p, connected: result.success, error: result.error, lastChecked: new Date().toISOString() }
          : p
      ));

      return result.success;
    } catch (err) {
      setProviders(prev => prev.map(p =>
        p.provider === providerId
          ? { ...p, connected: false, error: 'Connection test failed' }
          : p
      ));
      return false;
    } finally {
      setTesting(null);
    }
  };

  const handleProviderClick = (providerId: string) => {
    const provider = providers.find(p => p.provider === providerId);
    if (provider && provider.connected && onProviderSelect) {
      onProviderSelect(providerId);
    }
  };

  const handleAddProvider = () => {
    setShowAddModal(true);
    setSelectedProvider('');
    setProviderConfig({});
  };

  const handleSaveProvider = async () => {
    if (!selectedProvider) return;

    setSaving(true);
    try {
      const response = await fetch('/api/providers/add', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          provider: selectedProvider,
          config: providerConfig,
        }),
      });

      if (response.ok) {
        await loadProviders();
        setShowAddModal(false);
        setSelectedProvider('');
        setProviderConfig({});
      }
    } catch (err) {
      console.error('Failed to save provider:', err);
    } finally {
      setSaving(false);
    }
  };

  const getProviderFields = (providerId: string): { name: string; label: string; type: string; required: boolean }[] => {
    const fields: { [key: string]: { name: string; label: string; type: string; required: boolean }[] } = {
      vsphere: [
        { name: 'host', label: 'vCenter Host', type: 'text', required: true },
        { name: 'username', label: 'Username', type: 'text', required: true },
        { name: 'password', label: 'Password', type: 'password', required: true },
        { name: 'datacenter', label: 'Datacenter', type: 'text', required: false },
        { name: 'insecure', label: 'Allow Insecure SSL', type: 'checkbox', required: false },
      ],
      aws: [
        { name: 'access_key', label: 'Access Key ID', type: 'text', required: true },
        { name: 'secret_key', label: 'Secret Access Key', type: 'password', required: true },
        { name: 'region', label: 'Region', type: 'text', required: true },
      ],
      azure: [
        { name: 'subscription_id', label: 'Subscription ID', type: 'text', required: true },
        { name: 'tenant_id', label: 'Tenant ID', type: 'text', required: true },
        { name: 'client_id', label: 'Client ID', type: 'text', required: true },
        { name: 'client_secret', label: 'Client Secret', type: 'password', required: true },
        { name: 'resource_group', label: 'Resource Group', type: 'text', required: false },
      ],
      gcp: [
        { name: 'project_id', label: 'Project ID', type: 'text', required: true },
        { name: 'credentials_json', label: 'Service Account JSON', type: 'textarea', required: true },
        { name: 'zone', label: 'Default Zone', type: 'text', required: false },
      ],
      hyperv: [
        { name: 'host', label: 'Hyper-V Host', type: 'text', required: true },
        { name: 'username', label: 'Username', type: 'text', required: true },
        { name: 'password', label: 'Password', type: 'password', required: true },
        { name: 'use_winrm', label: 'Use WinRM', type: 'checkbox', required: false },
      ],
      oci: [
        { name: 'tenancy_ocid', label: 'Tenancy OCID', type: 'text', required: true },
        { name: 'user_ocid', label: 'User OCID', type: 'text', required: true },
        { name: 'fingerprint', label: 'Fingerprint', type: 'text', required: true },
        { name: 'private_key', label: 'Private Key', type: 'textarea', required: true },
        { name: 'region', label: 'Region', type: 'text', required: true },
      ],
      openstack: [
        { name: 'auth_url', label: 'Auth URL', type: 'text', required: true },
        { name: 'username', label: 'Username', type: 'text', required: true },
        { name: 'password', label: 'Password', type: 'password', required: true },
        { name: 'tenant_name', label: 'Tenant Name', type: 'text', required: true },
        { name: 'domain_name', label: 'Domain Name', type: 'text', required: false },
        { name: 'region', label: 'Region', type: 'text', required: false },
      ],
      alibabacloud: [
        { name: 'access_key_id', label: 'Access Key ID', type: 'text', required: true },
        { name: 'access_key_secret', label: 'Access Key Secret', type: 'password', required: true },
        { name: 'region', label: 'Region', type: 'text', required: true },
      ],
      proxmox: [
        { name: 'host', label: 'Proxmox Host', type: 'text', required: true },
        { name: 'port', label: 'Port', type: 'number', required: false },
        { name: 'username', label: 'Username', type: 'text', required: true },
        { name: 'password', label: 'Password', type: 'password', required: true },
        { name: 'node', label: 'Node', type: 'text', required: false },
      ],
    };

    return fields[providerId] || [];
  };

  return (
    <div style={styles.container}>
      <div style={styles.header}>
        <h2 style={styles.title}>Provider Connections</h2>
        <button onClick={handleAddProvider} style={styles.addButton}>
          ➕ Add Provider
        </button>
      </div>

      {/* Provider Grid */}
      <div style={styles.providerGrid}>
        {providers.map((provider) => {
          const providerInfo = AVAILABLE_PROVIDERS.find(p => p.id === provider.provider);
          return (
            <div
              key={provider.provider}
              style={{
                ...styles.providerCard,
                ...(provider.connected ? styles.providerCardConnected : styles.providerCardDisconnected)
              }}
              onClick={() => handleProviderClick(provider.provider)}
            >
              <div style={styles.providerCardHeader}>
                <span style={styles.providerIcon}>{providerInfo?.icon || '📦'}</span>
                <div style={styles.providerCardInfo}>
                  <h3 style={styles.providerName}>{providerInfo?.name || provider.provider}</h3>
                  <span style={{
                    ...styles.statusBadge,
                    backgroundColor: provider.connected ? '#4caf50' : '#f44336'
                  }}>
                    {provider.connected ? '✓ Connected' : '✗ Disconnected'}
                  </span>
                </div>
              </div>

              {provider.error && (
                <div style={styles.providerError}>
                  ⚠️ {provider.error}
                </div>
              )}

              {provider.lastChecked && (
                <div style={styles.providerLastChecked}>
                  Last checked: {new Date(provider.lastChecked).toLocaleString()}
                </div>
              )}

              <div style={styles.providerActions}>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    testConnection(provider.provider);
                  }}
                  disabled={testing === provider.provider}
                  style={{
                    ...styles.testButton,
                    ...(testing === provider.provider ? styles.testButtonDisabled : {})
                  }}
                >
                  {testing === provider.provider ? '🔄 Testing...' : '🔌 Test Connection'}
                </button>
                {provider.connected && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      handleProviderClick(provider.provider);
                    }}
                    style={styles.browseButton}
                  >
                    📂 Browse VMs
                  </button>
                )}
              </div>
            </div>
          );
        })}

        {/* Empty State */}
        {providers.length === 0 && (
          <div style={styles.emptyState}>
            <p style={styles.emptyIcon}>🔌</p>
            <p style={styles.emptyText}>No providers configured</p>
            <p style={styles.emptySubtext}>Click "Add Provider" to get started</p>
          </div>
        )}
      </div>

      {/* Add Provider Modal */}
      {showAddModal && (
        <div style={styles.modal} onClick={() => setShowAddModal(false)}>
          <div style={styles.modalContent} onClick={(e) => e.stopPropagation()}>
            <div style={styles.modalHeader}>
              <h2 style={styles.modalTitle}>Add Cloud Provider</h2>
              <button onClick={() => setShowAddModal(false)} style={styles.closeButton}>
                ✕
              </button>
            </div>

            <div style={styles.modalBody}>
              {/* Provider Selection */}
              <div style={styles.formGroup}>
                <label style={styles.label}>Select Provider:</label>
                <select
                  value={selectedProvider}
                  onChange={(e) => {
                    setSelectedProvider(e.target.value);
                    setProviderConfig({});
                  }}
                  style={styles.select}
                >
                  <option value="">-- Choose a provider --</option>
                  {AVAILABLE_PROVIDERS.map(p => (
                    <option key={p.id} value={p.id}>
                      {p.icon} {p.name}
                    </option>
                  ))}
                </select>
              </div>

              {/* Provider-Specific Fields */}
              {selectedProvider && (
                <>
                  {getProviderFields(selectedProvider).map((field) => (
                    <div key={field.name} style={styles.formGroup}>
                      <label style={styles.label}>
                        {field.label}
                        {field.required && <span style={styles.required}>*</span>}
                      </label>
                      {field.type === 'textarea' ? (
                        <textarea
                          value={providerConfig[field.name] || ''}
                          onChange={(e) => setProviderConfig({ ...providerConfig, [field.name]: e.target.value })}
                          style={styles.textarea}
                          rows={4}
                        />
                      ) : field.type === 'checkbox' ? (
                        <input
                          type="checkbox"
                          checked={providerConfig[field.name] === 'true'}
                          onChange={(e) => setProviderConfig({ ...providerConfig, [field.name]: e.target.checked.toString() })}
                          style={styles.checkbox}
                        />
                      ) : (
                        <input
                          type={field.type}
                          value={providerConfig[field.name] || ''}
                          onChange={(e) => setProviderConfig({ ...providerConfig, [field.name]: e.target.value })}
                          style={styles.input}
                        />
                      )}
                    </div>
                  ))}
                </>
              )}
            </div>

            <div style={styles.modalFooter}>
              <button onClick={() => setShowAddModal(false)} style={styles.cancelButton}>
                Cancel
              </button>
              <button
                onClick={handleSaveProvider}
                disabled={!selectedProvider || saving}
                style={{
                  ...styles.saveButton,
                  ...(!selectedProvider || saving ? styles.saveButtonDisabled : {})
                }}
              >
                {saving ? 'Saving...' : 'Save Provider'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

const styles: { [key: string]: React.CSSProperties } = {
  container: {
    padding: '20px',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: '20px',
  },
  title: {
    margin: 0,
    fontSize: '24px',
    color: '#222324',
  },
  addButton: {
    padding: '10px 20px',
    backgroundColor: '#f0583a',
    color: '#fff',
    border: 'none',
    borderRadius: '5px',
    cursor: 'pointer',
    fontSize: '14px',
    fontWeight: 'bold',
  },
  providerGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
    gap: '20px',
  },
  providerCard: {
    padding: '20px',
    border: '2px solid',
    borderRadius: '8px',
    cursor: 'pointer',
    transition: 'all 0.3s',
    backgroundColor: '#fff',
  },
  providerCardConnected: {
    borderColor: '#4caf50',
  },
  providerCardDisconnected: {
    borderColor: '#f44336',
  },
  providerCardHeader: {
    display: 'flex',
    alignItems: 'center',
    marginBottom: '15px',
  },
  providerIcon: {
    fontSize: '40px',
    marginRight: '15px',
  },
  providerCardInfo: {
    flex: 1,
  },
  providerName: {
    margin: '0 0 5px 0',
    fontSize: '18px',
    color: '#222324',
  },
  statusBadge: {
    display: 'inline-block',
    padding: '4px 8px',
    borderRadius: '12px',
    color: '#fff',
    fontSize: '12px',
    fontWeight: 'bold',
  },
  providerError: {
    padding: '10px',
    backgroundColor: '#ffebee',
    borderRadius: '4px',
    color: '#c62828',
    fontSize: '12px',
    marginBottom: '10px',
  },
  providerLastChecked: {
    fontSize: '12px',
    color: '#999',
    marginBottom: '10px',
  },
  providerActions: {
    display: 'flex',
    gap: '10px',
  },
  testButton: {
    flex: 1,
    padding: '8px',
    backgroundColor: '#2196f3',
    color: '#fff',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '12px',
    fontWeight: 'bold',
  },
  testButtonDisabled: {
    backgroundColor: '#ccc',
    cursor: 'not-allowed',
  },
  browseButton: {
    flex: 1,
    padding: '8px',
    backgroundColor: '#4caf50',
    color: '#fff',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '12px',
    fontWeight: 'bold',
  },
  emptyState: {
    gridColumn: '1 / -1',
    textAlign: 'center',
    padding: '60px 20px',
    color: '#999',
  },
  emptyIcon: {
    fontSize: '64px',
    margin: '0 0 10px 0',
  },
  emptyText: {
    fontSize: '18px',
    margin: '0 0 5px 0',
  },
  emptySubtext: {
    fontSize: '14px',
    margin: 0,
  },
  modal: {
    position: 'fixed',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    backgroundColor: 'rgba(0, 0, 0, 0.5)',
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    zIndex: 1000,
  },
  modalContent: {
    backgroundColor: '#fff',
    borderRadius: '8px',
    maxWidth: '600px',
    width: '90%',
    maxHeight: '80vh',
    display: 'flex',
    flexDirection: 'column',
  },
  modalHeader: {
    padding: '20px',
    borderBottom: '1px solid #eee',
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  modalTitle: {
    margin: 0,
    fontSize: '20px',
    color: '#222324',
  },
  closeButton: {
    background: 'none',
    border: 'none',
    fontSize: '24px',
    cursor: 'pointer',
    color: '#999',
  },
  modalBody: {
    padding: '20px',
    overflowY: 'auto',
    flex: 1,
  },
  formGroup: {
    marginBottom: '20px',
  },
  label: {
    display: 'block',
    marginBottom: '5px',
    fontWeight: 'bold',
    color: '#333',
    fontSize: '14px',
  },
  required: {
    color: '#f44336',
    marginLeft: '4px',
  },
  select: {
    width: '100%',
    padding: '10px',
    border: '1px solid #ddd',
    borderRadius: '5px',
    fontSize: '14px',
  },
  input: {
    width: '100%',
    padding: '10px',
    border: '1px solid #ddd',
    borderRadius: '5px',
    fontSize: '14px',
    boxSizing: 'border-box',
  },
  textarea: {
    width: '100%',
    padding: '10px',
    border: '1px solid #ddd',
    borderRadius: '5px',
    fontSize: '14px',
    fontFamily: 'monospace',
    boxSizing: 'border-box',
  },
  checkbox: {
    width: '20px',
    height: '20px',
    cursor: 'pointer',
  },
  modalFooter: {
    padding: '20px',
    borderTop: '1px solid #eee',
    display: 'flex',
    justifyContent: 'flex-end',
    gap: '10px',
  },
  cancelButton: {
    padding: '10px 20px',
    backgroundColor: '#ccc',
    color: '#333',
    border: 'none',
    borderRadius: '5px',
    cursor: 'pointer',
    fontSize: '14px',
    fontWeight: 'bold',
  },
  saveButton: {
    padding: '10px 20px',
    backgroundColor: '#f0583a',
    color: '#fff',
    border: 'none',
    borderRadius: '5px',
    cursor: 'pointer',
    fontSize: '14px',
    fontWeight: 'bold',
  },
  saveButtonDisabled: {
    backgroundColor: '#ccc',
    cursor: 'not-allowed',
  },
};

export default ProviderManager;
