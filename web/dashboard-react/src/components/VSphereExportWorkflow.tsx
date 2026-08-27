// SPDX-License-Identifier: Apache-2.0

import React, { useState, useEffect } from 'react';

interface VM {
  id: string;
  name: string;
  power_state: string;
  cpu_count?: number;
  memory_mb?: number;
  os?: string;
  ip_address?: string;
  datacenter?: string;
  cluster?: string;
}

type WorkflowStep = 'login' | 'discover' | 'export';

function isWindowsGuest(vm: VM | null): boolean {
  if (!vm) return false;
  const os = (vm.os || '').toLowerCase();
  if (os.includes('win') || os.includes('microsoft')) return true;
  const name = (vm.name || '').toLowerCase();
  return name.includes('win') || name.includes('windows');
}

const VSphereExportWorkflow: React.FC = () => {
  const [currentStep, setCurrentStep] = useState<WorkflowStep>('login');
  const [vSphereConfig, setVSphereConfig] = useState({
    vcenter: '',
    datacenter: '',
    username: '',
    password: '',
    insecure: true,
  });
  const [vms, setVms] = useState<VM[]>([]);
  const [selectedVM, setSelectedVM] = useState<VM | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showPassword, setShowPassword] = useState(false);
  const [rememberVSphere, setRememberVSphere] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const vmsPerPage = 12;

  // Load saved vSphere credentials on mount
  useEffect(() => {
    const savedVCenter = localStorage.getItem('vsphere_vcenter');
    const savedDatacenter = localStorage.getItem('vsphere_datacenter');
    const savedUsername = localStorage.getItem('vsphere_username');
    const savedPassword = localStorage.getItem('vsphere_password');
    const savedInsecure = localStorage.getItem('vsphere_insecure');

    if (savedVCenter && savedUsername && savedPassword) {
      setVSphereConfig({
        vcenter: savedVCenter,
        datacenter: savedDatacenter || '',
        username: savedUsername,
        password: savedPassword,
        insecure: savedInsecure === 'true',
      });
      setRememberVSphere(true);
    }
  }, []);

  // Export options
  const [exportOptions, setExportOptions] = useState({
    jobName: '',
    outputDir: '/tmp/exports',
    format: 'ova',
    compress: false,
    enablePipeline: false,
    enableRdp: true,
  });

  // Step 1: vSphere Login
  const handleVSphereLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      // Trim all inputs to remove any whitespace (including non-breaking spaces from copy-paste)
      const cleanedConfig = {
        vcenter: vSphereConfig.vcenter.trim().replace(/\s+/g, ''),
        datacenter: vSphereConfig.datacenter.trim(),
        username: vSphereConfig.username.trim(),
        password: vSphereConfig.password.trim(),
        insecure: vSphereConfig.insecure,
      };

      // Call API to discover VMs from vSphere (backend expects GET with query params)
      const params = new URLSearchParams({
        server: cleanedConfig.vcenter,
        username: cleanedConfig.username,
        password: cleanedConfig.password,
        insecure: cleanedConfig.insecure.toString(),
      });

      const response = await fetch(`/vms/list?${params}`, {
        method: 'GET',
      });

      if (!response.ok) {
        const errorText = await response.text();
        console.error('API Error:', response.status, errorText);
        throw new Error(errorText || 'Failed to connect to vSphere. Please check your credentials.');
      }

      const discoveredVMs = await response.json();
      console.log('Discovered VMs Response:', discoveredVMs);

      // Backend returns { vms: [...], total: N, timestamp: ... }
      const vmList = discoveredVMs.vms || discoveredVMs.VMs || [];
      console.log('VM List:', vmList);
      setVms(Array.isArray(vmList) ? vmList : []);
      setCurrentPage(1); // Reset to first page

      // Update state with cleaned values
      setVSphereConfig(cleanedConfig);

      // Save vSphere credentials if "Remember" is checked
      if (rememberVSphere) {
        localStorage.setItem('vsphere_vcenter', cleanedConfig.vcenter);
        localStorage.setItem('vsphere_datacenter', cleanedConfig.datacenter);
        localStorage.setItem('vsphere_username', cleanedConfig.username);
        localStorage.setItem('vsphere_password', cleanedConfig.password);
        localStorage.setItem('vsphere_insecure', cleanedConfig.insecure.toString());
      } else {
        localStorage.removeItem('vsphere_vcenter');
        localStorage.removeItem('vsphere_datacenter');
        localStorage.removeItem('vsphere_username');
        localStorage.removeItem('vsphere_password');
        localStorage.removeItem('vsphere_insecure');
      }

      setCurrentStep('discover');
    } catch (err) {
      console.error('vSphere login error:', err);
      setError(err instanceof Error ? err.message : 'Connection failed');
    } finally {
      setLoading(false);
    }
  };

  // Step 2: Select VM
  const handleVMSelect = (vm: VM) => {
    setSelectedVM(vm);
    const windows = isWindowsGuest(vm);
    setExportOptions(prev => ({
      ...prev,
      jobName: vm.name || '',
      enableRdp: windows ? true : prev.enableRdp,
    }));
    setCurrentStep('export');
  };

  // Step 3: Submit Export Job
  const handleExportSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      const jobData = {
        provider: 'vsphere',
        vm_identifier: selectedVM?.id,
        vm_name: selectedVM?.name,
        name: exportOptions.jobName,
        output_dir: exportOptions.outputDir,
        format: exportOptions.format,
        compress: exportOptions.compress,
        enable_pipeline: exportOptions.enablePipeline,
        enable_rdp: exportOptions.enableRdp,
        options: {
          enable_pipeline: exportOptions.enablePipeline,
          enable_rdp: exportOptions.enableRdp,
        },
        // vSphere connection details
        vsphere_config: vSphereConfig,
      };

      const response = await fetch('/jobs/submit', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(jobData),
      });

      if (!response.ok) {
        throw new Error('Failed to submit export job');
      }

      const result = await response.json();
      alert(`Job submitted successfully! Job ID: ${result.job_id || 'N/A'}`);

      // Reset workflow
      setCurrentStep('login');
      setSelectedVM(null);
      setVms([]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Export failed');
    } finally {
      setLoading(false);
    }
  };

  const handleBackToVMs = () => {
    setCurrentStep('discover');
    setSelectedVM(null);
  };

  const handleBackToLogin = () => {
    setCurrentStep('login');
    setVms([]);
    setSelectedVM(null);
    setCurrentPage(1);
  };

  const getStatusColor = (status: string): string => {
    const s = status.toLowerCase();
    if (s.includes('on') || s.includes('running')) return '#4caf50';
    if (s.includes('off') || s.includes('stopped')) return '#f44336';
    return '#ff9800';
  };

  // Render Step 1: vSphere Login
  const renderLoginStep = () => (
    <div style={styles.stepContainer}>
      <h2 style={styles.stepTitle}>Connect to vSphere</h2>
      <p style={styles.stepDescription}>
        Enter your vCenter credentials to discover virtual machines
      </p>

      <form onSubmit={handleVSphereLogin} style={styles.form}>
        <div style={styles.formGrid}>
          <div style={styles.formGroup}>
            <input
              type="text"
              value={vSphereConfig.vcenter}
              onChange={(e) => setVSphereConfig({ ...vSphereConfig, vcenter: e.target.value })}
              placeholder="vCenter Server *"
              required
              style={styles.input}
              onFocus={(e) => {
                e.currentTarget.style.borderColor = '#f0583a';
              }}
              onBlur={(e) => {
                setVSphereConfig({ ...vSphereConfig, vcenter: e.target.value.trim().replace(/\s+/g, '') });
                e.currentTarget.style.borderColor = '#e5e7eb';
              }}
            />
          </div>

          <div style={styles.formGroup}>
            <input
              type="text"
              value={vSphereConfig.datacenter}
              onChange={(e) => setVSphereConfig({ ...vSphereConfig, datacenter: e.target.value })}
              placeholder="Datacenter (optional)"
              style={styles.input}
              onFocus={(e) => {
                e.currentTarget.style.borderColor = '#f0583a';
              }}
              onBlur={(e) => {
                setVSphereConfig({ ...vSphereConfig, datacenter: e.target.value.trim() });
                e.currentTarget.style.borderColor = '#e5e7eb';
              }}
            />
          </div>

          <div style={styles.formGroup}>
            <input
              type="text"
              value={vSphereConfig.username}
              onChange={(e) => setVSphereConfig({ ...vSphereConfig, username: e.target.value })}
              placeholder="Username *"
              required
              style={styles.input}
              onFocus={(e) => {
                e.currentTarget.style.borderColor = '#f0583a';
              }}
              onBlur={(e) => {
                setVSphereConfig({ ...vSphereConfig, username: e.target.value.trim() });
                e.currentTarget.style.borderColor = '#e5e7eb';
              }}
            />
          </div>

          <div style={styles.formGroup}>
            <input
              type={showPassword ? 'text' : 'password'}
              value={vSphereConfig.password}
              onChange={(e) => setVSphereConfig({ ...vSphereConfig, password: e.target.value })}
              placeholder="Password *"
              required
              style={styles.input}
              onFocus={(e) => {
                e.currentTarget.style.borderColor = '#f0583a';
              }}
              onBlur={(e) => {
                setVSphereConfig({ ...vSphereConfig, password: e.target.value.trim() });
                e.currentTarget.style.borderColor = '#e5e7eb';
              }}
            />
          </div>
        </div>

        <div style={{ ...styles.formGroupFull, marginTop: '12px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
          <label style={styles.checkboxLabel}>
            <input
              type="checkbox"
              checked={showPassword}
              onChange={(e) => setShowPassword(e.target.checked)}
              style={styles.checkbox}
            />
            Show password
          </label>

          <label style={styles.checkboxLabel}>
            <input
              type="checkbox"
              checked={vSphereConfig.insecure}
              onChange={(e) => setVSphereConfig({ ...vSphereConfig, insecure: e.target.checked })}
              style={styles.checkbox}
            />
            Skip SSL verification
          </label>

          <label style={styles.checkboxLabel}>
            <input
              type="checkbox"
              checked={rememberVSphere}
              onChange={(e) => setRememberVSphere(e.target.checked)}
              style={styles.checkbox}
            />
            Remember credentials
          </label>
        </div>

        {error && (
          <div style={styles.errorBox}>
            <strong>⚠️ Error:</strong> {error}
          </div>
        )}

        <button
          type="submit"
          disabled={loading}
          style={{
            ...styles.submitButton,
            ...(loading ? styles.submitButtonDisabled : {}),
          }}
          onMouseEnter={(e) => {
            if (!loading) {
              e.currentTarget.style.backgroundColor = '#d94b32';
            }
          }}
          onMouseLeave={(e) => {
            if (!loading) {
              e.currentTarget.style.backgroundColor = '#f0583a';
            }
          }}
        >
          {loading ? 'Connecting...' : 'Discover VMs'}
        </button>
      </form>
    </div>
  );

  // Render Step 2: VM Discovery
  const renderDiscoverStep = () => {
    // Pagination logic
    const indexOfLastVM = currentPage * vmsPerPage;
    const indexOfFirstVM = indexOfLastVM - vmsPerPage;
    const currentVMs = vms.slice(indexOfFirstVM, indexOfLastVM);
    const totalPages = Math.ceil(vms.length / vmsPerPage);

    const handlePageChange = (pageNumber: number) => {
      setCurrentPage(pageNumber);
      window.scrollTo({ top: 0, behavior: 'smooth' });
    };

    return (
      <div style={styles.stepContainer}>
        <div style={styles.stepHeader}>
          <div>
            <h2 style={styles.stepTitle}>Step 2: Select Virtual Machine</h2>
            <p style={styles.stepDescription}>
              {vms.length} VM{vms.length !== 1 ? 's' : ''} discovered from {vSphereConfig.vcenter}
              {totalPages > 1 && ` • Page ${currentPage} of ${totalPages}`}
            </p>
          </div>
          <button onClick={handleBackToLogin} style={styles.backButton}>
            ← Back to Login
          </button>
        </div>

        {vms.length === 0 ? (
          <div style={styles.emptyState}>
            <p style={styles.emptyIcon}>📭</p>
            <p style={styles.emptyText}>No virtual machines found</p>
          </div>
        ) : (
          <>
            <div style={styles.vmGrid}>
              {currentVMs.map((vm) => (
            <div
              key={vm.id}
              style={styles.vmCard}
              onClick={() => handleVMSelect(vm)}
              onMouseEnter={(e) => {
                e.currentTarget.style.boxShadow = '0 8px 16px rgba(0,0,0,0.12)';
                e.currentTarget.style.transform = 'translateY(-4px)';
                e.currentTarget.style.borderColor = '#f0583a';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.boxShadow = '0 1px 3px rgba(0,0,0,0.08)';
                e.currentTarget.style.transform = 'translateY(0)';
                e.currentTarget.style.borderColor = '#e5e7eb';
              }}
            >
              <div style={styles.vmCardHeader}>
                <h3 style={styles.vmCardTitle}>{vm.name}</h3>
                <span
                  style={{
                    ...styles.statusBadge,
                    backgroundColor: getStatusColor(vm.power_state),
                  }}
                >
                  {vm.power_state}
                </span>
              </div>

              <div style={styles.vmCardBody}>
                {vm.cpu_count && (
                  <div style={styles.vmCardRow}>
                    <span style={styles.vmCardLabel}>CPU</span>
                    <span style={styles.vmCardValue}>{vm.cpu_count} vCPU</span>
                  </div>
                )}
                {vm.memory_mb && (
                  <div style={styles.vmCardRow}>
                    <span style={styles.vmCardLabel}>Memory</span>
                    <span style={styles.vmCardValue}>
                      {(vm.memory_mb / 1024).toFixed(1)} GB
                    </span>
                  </div>
                )}
                {vm.os && (
                  <div style={styles.vmCardRow}>
                    <span style={styles.vmCardLabel}>OS</span>
                    <span style={styles.vmCardValue}>{vm.os}</span>
                  </div>
                )}
                {vm.ip_address && (
                  <div style={styles.vmCardRow}>
                    <span style={styles.vmCardLabel}>IP</span>
                    <span style={styles.vmCardValue}>{vm.ip_address}</span>
                  </div>
                )}
              </div>

              <button
                style={styles.selectButton}
                onClick={(e) => {
                  e.stopPropagation();
                  handleVMSelect(vm);
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.backgroundColor = '#d94b32';
                  e.currentTarget.style.transform = 'scale(1.02)';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.backgroundColor = '#f0583a';
                  e.currentTarget.style.transform = 'scale(1)';
                }}
              >
                Select VM →
              </button>
            </div>
          ))}
        </div>

        {/* Pagination Controls */}
        {totalPages > 1 && (
          <div style={styles.pagination}>
            <button
              onClick={() => handlePageChange(currentPage - 1)}
              disabled={currentPage === 1}
              style={{
                ...styles.paginationButton,
                ...(currentPage === 1 ? styles.paginationButtonDisabled : {}),
              }}
            >
              ← Previous
            </button>

            <div style={styles.paginationNumbers}>
              {Array.from({ length: totalPages }, (_, i) => i + 1).map((pageNum) => {
                // Show first page, last page, current page, and pages around current
                const showPage =
                  pageNum === 1 ||
                  pageNum === totalPages ||
                  (pageNum >= currentPage - 1 && pageNum <= currentPage + 1);

                if (!showPage) {
                  // Show ellipsis
                  if (pageNum === currentPage - 2 || pageNum === currentPage + 2) {
                    return (
                      <span key={pageNum} style={styles.paginationEllipsis}>
                        ...
                      </span>
                    );
                  }
                  return null;
                }

                return (
                  <button
                    key={pageNum}
                    onClick={() => handlePageChange(pageNum)}
                    style={{
                      ...styles.paginationNumber,
                      ...(currentPage === pageNum ? styles.paginationNumberActive : {}),
                    }}
                  >
                    {pageNum}
                  </button>
                );
              })}
            </div>

            <button
              onClick={() => handlePageChange(currentPage + 1)}
              disabled={currentPage === totalPages}
              style={{
                ...styles.paginationButton,
                ...(currentPage === totalPages ? styles.paginationButtonDisabled : {}),
              }}
            >
              Next →
            </button>
          </div>
        )}
      </>
      )}
    </div>
    );
  };

  // Render Step 3: Export Options
  const renderExportStep = () => (
    <div style={styles.stepContainer}>
      <div style={styles.stepHeader}>
        <div>
          <h2 style={styles.stepTitle}>Step 3: Configure Export Options</h2>
          <p style={styles.stepDescription}>
            Export {selectedVM?.name} from vSphere to local storage
          </p>
        </div>
        <button onClick={handleBackToVMs} style={styles.backButton}>
          ← Back to VMs
        </button>
      </div>

      {/* Selected VM Summary */}
      <div style={styles.selectedVMBox}>
        <h3 style={styles.selectedVMTitle}>✓ Selected Virtual Machine</h3>
        <div style={styles.selectedVMGrid}>
          <div><strong>Name:</strong> {selectedVM?.name}</div>
          <div><strong>ID:</strong> {selectedVM?.id}</div>
          <div><strong>Status:</strong> {selectedVM?.power_state}</div>
          {selectedVM?.datacenter && <div><strong>Datacenter:</strong> {selectedVM.datacenter}</div>}
        </div>
      </div>

      {/* Export Options Form */}
      <form onSubmit={handleExportSubmit} style={styles.form}>
        <h3 style={styles.sectionTitle}>Export Options</h3>

        <div style={styles.formGroup}>
          <label style={styles.label}>
            Job Name <span style={styles.required}>*</span>
          </label>
          <input
            type="text"
            value={exportOptions.jobName}
            onChange={(e) => setExportOptions({ ...exportOptions, jobName: e.target.value })}
            placeholder="my-export-job"
            required
            style={styles.input}
          />
        </div>

        <div style={styles.formGroup}>
          <label style={styles.label}>
            Output Directory <span style={styles.required}>*</span>
          </label>
          <input
            type="text"
            value={exportOptions.outputDir}
            onChange={(e) => setExportOptions({ ...exportOptions, outputDir: e.target.value })}
            placeholder="/tmp/exports"
            required
            style={styles.input}
          />
        </div>

        <div style={styles.formGroup}>
          <label style={styles.label}>
            Format <span style={styles.required}>*</span>
          </label>
          <select
            value={exportOptions.format}
            onChange={(e) => setExportOptions({ ...exportOptions, format: e.target.value })}
            style={styles.input}
          >
            <option value="ova">OVA (Open Virtual Appliance)</option>
            <option value="ovf">OVF (Open Virtualization Format)</option>
            <option value="vmdk">VMDK (Virtual Machine Disk)</option>
          </select>
        </div>

        <div style={styles.formGroup}>
          <label style={styles.checkboxLabel}>
            <input
              type="checkbox"
              checked={exportOptions.compress}
              onChange={(e) => setExportOptions({ ...exportOptions, compress: e.target.checked })}
              style={styles.checkbox}
            />
            Compress output
          </label>
        </div>

        <h3 style={styles.sectionTitle}>Pipeline Integration (h2kvm + libvirt)</h3>

        <div style={styles.formGroup}>
          <label style={styles.checkboxLabel}>
            <input
              type="checkbox"
              checked={exportOptions.enablePipeline}
              onChange={(e) => setExportOptions({ ...exportOptions, enablePipeline: e.target.checked })}
              style={styles.checkbox}
            />
            Enable h2kvm pipeline after export
          </label>
          <p style={styles.helpText}>
            Automatically convert exported VM to KVM format and import to libvirt
          </p>
        </div>

        {exportOptions.enablePipeline && isWindowsGuest(selectedVM) && (
          <div style={styles.formGroup}>
            <label style={styles.checkboxLabel}>
              <input
                type="checkbox"
                checked={exportOptions.enableRdp}
                onChange={(e) => setExportOptions({ ...exportOptions, enableRdp: e.target.checked })}
                style={styles.checkbox}
              />
              Enable Remote Desktop (RDP) on first boot
            </label>
            <p style={styles.helpText}>
              Passed to h2kvm fix stage (registry, firewall, TermService). Default on for Windows.
            </p>
          </div>
        )}

        {error && (
          <div style={styles.errorBox}>
            <strong>⚠️ Error:</strong> {error}
          </div>
        )}

        <button
          type="submit"
          disabled={loading}
          style={{
            ...styles.submitButton,
            ...(loading ? styles.submitButtonDisabled : {}),
          }}
        >
          {loading ? '⏳ Submitting...' : '📤 Submit Export Job'}
        </button>
      </form>
    </div>
  );

  return (
    <div style={styles.container}>
      {/* Progress Indicator */}
      <div style={styles.progressBar}>
        <div style={{
          ...styles.progressStep,
          ...(currentStep === 'login' ? styles.progressStepActive : styles.progressStepComplete)
        }}>
          {currentStep === 'login' ? '1' : '✓'} vSphere Login
        </div>
        <div style={styles.progressLine} />
        <div style={{
          ...styles.progressStep,
          ...(currentStep === 'discover' ? styles.progressStepActive : currentStep === 'export' ? styles.progressStepComplete : {})
        }}>
          {currentStep === 'export' ? '✓' : '2'} Discover VMs
        </div>
        <div style={styles.progressLine} />
        <div style={{
          ...styles.progressStep,
          ...(currentStep === 'export' ? styles.progressStepActive : {})
        }}>
          3 Export Options
        </div>
      </div>

      {/* Step Content */}
      {currentStep === 'login' && renderLoginStep()}
      {currentStep === 'discover' && renderDiscoverStep()}
      {currentStep === 'export' && renderExportStep()}
    </div>
  );
};

const styles: { [key: string]: React.CSSProperties } = {
  container: {
    padding: '0',
    maxWidth: '1200px',
    margin: '0 auto',
    backgroundColor: '#f0f2f7',
  },
  progressBar: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: '0',
    padding: '24px 32px',
    backgroundColor: '#fff',
    borderRadius: '0',
    border: 'none',
    borderBottom: '1px solid #e5e7eb',
  },
  progressStep: {
    padding: '10px 20px',
    backgroundColor: 'transparent',
    color: '#9ca3af',
    borderRadius: '0',
    fontWeight: '600',
    fontSize: '14px',
    transition: 'all 0.25s cubic-bezier(0.215, 0.61, 0.355, 1)',
    borderBottom: '3px solid transparent',
  },
  progressStepActive: {
    backgroundColor: 'transparent',
    color: '#000',
    borderBottom: '3px solid #f0583a',
    boxShadow: 'none',
  },
  progressStepComplete: {
    backgroundColor: 'transparent',
    color: '#000',
  },
  progressLine: {
    display: 'none',
  },
  stepContainer: {
    backgroundColor: '#fff',
    padding: '32px',
    borderRadius: '0',
    boxShadow: 'none',
    border: 'none',
  },
  stepHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    marginBottom: '24px',
    paddingBottom: '0',
    borderBottom: 'none',
  },
  stepTitle: {
    margin: '0 0 8px 0',
    fontSize: '28px',
    color: '#000',
    fontWeight: '700',
    letterSpacing: '-0.5px',
  },
  stepDescription: {
    margin: 0,
    fontSize: '14px',
    color: '#6b7280',
    fontWeight: '400',
    lineHeight: '1.6',
  },
  backButton: {
    padding: '6px 12px',
    backgroundColor: '#6b7280',
    color: '#fff',
    border: 'none',
    borderRadius: '5px',
    cursor: 'pointer',
    fontSize: '12px',
    fontWeight: '600',
    transition: 'all 0.2s',
  },
  form: {
    marginTop: '20px',
    maxWidth: '100%',
  },
  formGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(2, 1fr)',
    gap: '16px',
    marginBottom: '16px',
  },
  formGroup: {
    marginBottom: '0',
    position: 'relative',
  },
  formGroupFull: {
    gridColumn: '1 / -1',
  },
  label: {
    position: 'absolute',
    left: '14px',
    top: '50%',
    transform: 'translateY(-50%)',
    fontSize: '14px',
    color: '#9ca3af',
    pointerEvents: 'none',
    transition: 'all 0.25s cubic-bezier(0.215, 0.61, 0.355, 1)',
    backgroundColor: '#fff',
    padding: '0 4px',
  },
  labelFocused: {
    top: '0',
    fontSize: '11px',
    color: '#f0583a',
    fontWeight: '600',
  },
  required: {
    color: '#f0583a',
  },
  input: {
    width: '100%',
    padding: '14px',
    border: '1px solid #e5e7eb',
    borderRadius: '0',
    fontSize: '14px',
    boxSizing: 'border-box',
    transition: 'all 0.25s cubic-bezier(0.215, 0.61, 0.355, 1)',
    backgroundColor: '#fff',
    fontFamily: 'inherit',
    outline: 'none',
  },
  checkboxLabel: {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    fontSize: '13px',
    color: '#000',
    fontWeight: '400',
    cursor: 'pointer',
    userSelect: 'none',
  },
  checkbox: {
    width: '16px',
    height: '16px',
    cursor: 'pointer',
    accentColor: '#f0583a',
  },
  helpText: {
    margin: '4px 0 0 20px',
    fontSize: '10px',
    color: '#9ca3af',
    fontStyle: 'italic',
  },
  sectionTitle: {
    margin: '16px 0 10px 0',
    fontSize: '14px',
    color: '#111827',
    fontWeight: '600',
    borderBottom: '1px solid #e5e7eb',
    paddingBottom: '6px',
  },
  errorBox: {
    padding: '7px 10px',
    backgroundColor: '#fee2e2',
    border: '1px solid #fecaca',
    borderRadius: '4px',
    color: '#991b1b',
    marginBottom: '10px',
    fontSize: '11px',
    fontWeight: '500',
  },
  submitButton: {
    width: 'auto',
    padding: '14px 32px',
    backgroundColor: '#f0583a',
    color: '#fff',
    border: 'none',
    borderRadius: '0',
    cursor: 'pointer',
    fontSize: '14px',
    fontWeight: '600',
    marginTop: '24px',
    transition: 'all 0.25s cubic-bezier(0.215, 0.61, 0.355, 1)',
    textTransform: 'uppercase',
    letterSpacing: '1px',
  },
  submitButtonDisabled: {
    backgroundColor: '#9ca3af',
    cursor: 'not-allowed',
    opacity: 0.6,
  },
  vmGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
    gap: '20px',
    marginTop: '24px',
  },
  vmCard: {
    padding: '18px',
    border: '1px solid #e5e7eb',
    borderRadius: '8px',
    cursor: 'pointer',
    transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
    backgroundColor: '#fff',
    boxShadow: '0 1px 3px rgba(0,0,0,0.08)',
    position: 'relative',
    overflow: 'hidden',
  },
  vmCardHeader: {
    marginBottom: '16px',
    paddingBottom: '12px',
    borderBottom: '2px solid #f3f4f6',
  },
  vmCardTitle: {
    margin: '0 0 8px 0',
    fontSize: '15px',
    color: '#111827',
    fontWeight: '700',
    lineHeight: '1.4',
  },
  statusBadge: {
    display: 'inline-block',
    padding: '4px 10px',
    borderRadius: '12px',
    color: '#fff',
    fontSize: '11px',
    fontWeight: '600',
    textTransform: 'uppercase',
    letterSpacing: '0.5px',
  },
  vmCardBody: {
    marginBottom: '16px',
    display: 'flex',
    flexDirection: 'column',
    gap: '10px',
  },
  vmCardRow: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '8px 12px',
    backgroundColor: '#f9fafb',
    borderRadius: '6px',
  },
  vmCardLabel: {
    fontWeight: '600',
    color: '#6b7280',
    fontSize: '12px',
    textTransform: 'uppercase',
    letterSpacing: '0.5px',
  },
  vmCardValue: {
    color: '#111827',
    fontSize: '13px',
    fontWeight: '500',
  },
  selectButton: {
    width: '100%',
    padding: '6px 10px',
    backgroundColor: '#f0583a',
    color: '#fff',
    border: 'none',
    borderRadius: '5px',
    cursor: 'pointer',
    fontSize: '11px',
    fontWeight: '600',
    transition: 'all 0.2s',
    letterSpacing: '0.3px',
  },
  pagination: {
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    gap: '12px',
    marginTop: '32px',
    paddingTop: '24px',
    borderTop: '2px solid #f3f4f6',
  },
  paginationButton: {
    padding: '8px 16px',
    backgroundColor: '#f0583a',
    color: '#fff',
    border: 'none',
    borderRadius: '6px',
    cursor: 'pointer',
    fontSize: '13px',
    fontWeight: '600',
    transition: 'all 0.2s',
  },
  paginationButtonDisabled: {
    backgroundColor: '#d1d5db',
    cursor: 'not-allowed',
    opacity: 0.6,
  },
  paginationNumbers: {
    display: 'flex',
    gap: '6px',
    alignItems: 'center',
  },
  paginationNumber: {
    padding: '8px 12px',
    backgroundColor: '#fff',
    color: '#374151',
    border: '1px solid #e5e7eb',
    borderRadius: '6px',
    cursor: 'pointer',
    fontSize: '13px',
    fontWeight: '500',
    transition: 'all 0.2s',
    minWidth: '40px',
  },
  paginationNumberActive: {
    backgroundColor: '#f0583a',
    color: '#fff',
    borderColor: '#f0583a',
    fontWeight: '700',
  },
  paginationEllipsis: {
    padding: '8px 4px',
    color: '#6b7280',
    fontSize: '13px',
  },
  selectedVMBox: {
    padding: '12px 14px',
    backgroundColor: '#d1e7dd',
    border: '1px solid #a3cfbb',
    borderRadius: '6px',
    marginBottom: '16px',
  },
  selectedVMTitle: {
    margin: '0 0 10px 0',
    fontSize: '14px',
    color: '#0f5132',
    fontWeight: '600',
  },
  selectedVMGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
    gap: '8px',
    fontSize: '12px',
  },
  emptyState: {
    textAlign: 'center',
    padding: '40px 20px',
    color: '#6c757d',
  },
  emptyIcon: {
    fontSize: '48px',
    margin: '0 0 8px 0',
  },
  emptyText: {
    fontSize: '14px',
    margin: 0,
  },
};

export default VSphereExportWorkflow;
