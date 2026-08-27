// SPDX-License-Identifier: Apache-2.0

import React, { useEffect, useState } from 'react';
import { listNutanixVMs, submitNutanixExportJob } from '../utils/api';

interface NutanixVM {
  ID?: string;
  id?: string;
  Name?: string;
  name?: string;
  State?: string;
  state?: string;
  PowerState?: string;
  power_state?: string;
  NumCPUs?: number;
  num_cpus?: number;
  MemoryMB?: number;
  memory_mb?: number;
  Location?: string;
  location?: string;
  Metadata?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
}

type WorkflowStep = 'login' | 'discover' | 'export';

function vmField<T>(vm: NutanixVM, upper: keyof NutanixVM, lower: keyof NutanixVM, fallback: T): T {
  const upperVal = vm[upper] as T | undefined;
  const lowerVal = vm[lower] as T | undefined;
  return upperVal ?? lowerVal ?? fallback;
}

const NutanixExportWorkflow: React.FC = () => {
  const [currentStep, setCurrentStep] = useState<WorkflowStep>('login');
  const [config, setConfig] = useState({
    server: '',
    cluster: '',
    username: '',
    password: '',
    insecure: true,
  });
  const [vms, setVms] = useState<NutanixVM[]>([]);
  const [selectedVM, setSelectedVM] = useState<NutanixVM | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [remember, setRemember] = useState(false);
  const [search, setSearch] = useState('');
  const [exportOptions, setExportOptions] = useState({
    jobName: '',
    outputDir: '/var/lib/transiva/nutanix',
    format: 'qcow2',
    mounts: 'default-container:/mnt/nutanix/default',
    resolveContainers: true,
    enablePipeline: false,
  });

  useEffect(() => {
    const savedServer = localStorage.getItem('nutanix_server');
    const savedUser = localStorage.getItem('nutanix_username');
    const savedPass = localStorage.getItem('nutanix_password');
    const savedCluster = localStorage.getItem('nutanix_cluster');
    const savedInsecure = localStorage.getItem('nutanix_insecure');
    if (savedServer && savedUser && savedPass) {
      setConfig({
        server: savedServer,
        cluster: savedCluster || '',
        username: savedUser,
        password: savedPass,
        insecure: savedInsecure !== 'false',
      });
      setRemember(true);
    }
  }, []);

  const parseMounts = (spec: string): Record<string, string> => {
    const out: Record<string, string> = {};
    spec.split(',').forEach((part) => {
      const trimmed = part.trim();
      if (!trimmed) return;
      const idx = trimmed.indexOf(':');
      if (idx <= 0) return;
      const name = trimmed.slice(0, idx).trim();
      const path = trimmed.slice(idx + 1).trim();
      if (name && path) out[name] = path;
    });
    return out;
  };

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const cleaned = {
        server: config.server.trim(),
        cluster: config.cluster.trim(),
        username: config.username.trim(),
        password: config.password.trim(),
        insecure: config.insecure,
      };
      const resp = await listNutanixVMs(cleaned);
      setVms((resp.vms ?? []) as NutanixVM[]);
      setConfig(cleaned);
      if (remember) {
        localStorage.setItem('nutanix_server', cleaned.server);
        localStorage.setItem('nutanix_username', cleaned.username);
        localStorage.setItem('nutanix_password', cleaned.password);
        localStorage.setItem('nutanix_cluster', cleaned.cluster);
        localStorage.setItem('nutanix_insecure', String(cleaned.insecure));
      } else {
        ['nutanix_server', 'nutanix_username', 'nutanix_password', 'nutanix_cluster', 'nutanix_insecure']
          .forEach((k) => localStorage.removeItem(k));
      }
      setCurrentStep('discover');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Connection failed');
    } finally {
      setLoading(false);
    }
  };

  const handleVMSelect = (vm: NutanixVM) => {
    setSelectedVM(vm);
    setExportOptions((prev) => ({
      ...prev,
      jobName: vmField(vm, 'Name', 'name', 'nutanix-export'),
    }));
    setCurrentStep('export');
  };

  const handleExportSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedVM) return;
    setLoading(true);
    setError(null);
    try {
      const mounts = parseMounts(exportOptions.mounts);
      if (Object.keys(mounts).length === 0) {
        throw new Error('At least one mount is required (container:/path)');
      }
      const vmName = vmField(selectedVM, 'Name', 'name', '');
      const vmID = vmField(selectedVM, 'ID', 'id', vmName);
      const result = await submitNutanixExportJob({
        name: exportOptions.jobName || vmName,
        vm_path: vmID,
        output_path: exportOptions.outputDir,
        provider: 'nutanix',
        export_method: 'nutanix',
        format: exportOptions.format,
        enable_pipeline: exportOptions.enablePipeline,
        metadata: {
          mounts,
          resolve_containers: exportOptions.resolveContainers,
          server: config.server,
          username: config.username,
          password: config.password,
          insecure: config.insecure,
          cluster: config.cluster || undefined,
        },
      });
      const jobIDs = (result as { job_ids?: string[]; JobIDs?: string[] }).job_ids
        || (result as { job_ids?: string[]; JobIDs?: string[] }).JobIDs
        || [];
      alert(`Nutanix export job submitted${jobIDs.length ? `: ${jobIDs[0]}` : ''}`);
      setCurrentStep('login');
      setSelectedVM(null);
      setVms([]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Export failed');
    } finally {
      setLoading(false);
    }
  };

  const filtered = vms.filter((vm) => {
    const name = vmField(vm, 'Name', 'name', '').toLowerCase();
    const id = vmField(vm, 'ID', 'id', '').toLowerCase();
    const q = search.toLowerCase();
    return !q || name.includes(q) || id.includes(q);
  });

  if (currentStep === 'login') {
    return (
      <div style={styles.card}>
        <h2 style={styles.title}>Connect to Nutanix Prism</h2>
        <p style={styles.subtitle}>Discover VMs and export disks from NFS-mounted storage containers.</p>
        {error && <div style={styles.error}>{error}</div>}
        <form onSubmit={handleLogin} style={styles.form}>
          <input style={styles.input} placeholder="Prism Central host *" value={config.server} onChange={(e) => setConfig({ ...config, server: e.target.value })} required />
          <input style={styles.input} placeholder="Cluster filter (optional)" value={config.cluster} onChange={(e) => setConfig({ ...config, cluster: e.target.value })} />
          <input style={styles.input} placeholder="Username *" value={config.username} onChange={(e) => setConfig({ ...config, username: e.target.value })} required />
          <input style={styles.input} type="password" placeholder="Password *" value={config.password} onChange={(e) => setConfig({ ...config, password: e.target.value })} required />
          <label style={styles.checkbox}><input type="checkbox" checked={config.insecure} onChange={(e) => setConfig({ ...config, insecure: e.target.checked })} /> Allow insecure TLS</label>
          <label style={styles.checkbox}><input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} /> Remember credentials</label>
          <button style={styles.button} disabled={loading}>{loading ? 'Connecting...' : 'Discover VMs'}</button>
        </form>
      </div>
    );
  }

  if (currentStep === 'discover') {
    return (
      <div style={styles.card}>
        <div style={styles.headerRow}>
          <h2 style={styles.title}>Select VM ({filtered.length})</h2>
          <button style={styles.secondaryButton} onClick={() => setCurrentStep('login')}>Back</button>
        </div>
        <input style={styles.input} placeholder="Search by name or UUID" value={search} onChange={(e) => setSearch(e.target.value)} />
        {error && <div style={styles.error}>{error}</div>}
        <div style={styles.tableWrap}>
          <table style={styles.table}>
            <thead>
              <tr>
                <th style={styles.th}>Name</th>
                <th style={styles.th}>UUID</th>
                <th style={styles.th}>Power</th>
                <th style={styles.th}>vCPU</th>
                <th style={styles.th}>Memory</th>
                <th style={styles.th}></th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((vm) => {
                const id = vmField(vm, 'ID', 'id', '');
                return (
                  <tr key={id}>
                    <td style={styles.td}>{vmField(vm, 'Name', 'name', '')}</td>
                    <td style={styles.tdMono}>{id}</td>
                    <td style={styles.td}>{vmField(vm, 'PowerState', 'power_state', vmField(vm, 'State', 'state', ''))}</td>
                    <td style={styles.td}>{vmField(vm, 'NumCPUs', 'num_cpus', 0)}</td>
                    <td style={styles.td}>{Math.round(vmField(vm, 'MemoryMB', 'memory_mb', 0) / 1024)} GiB</td>
                    <td style={styles.td}><button style={styles.smallButton} onClick={() => handleVMSelect(vm)}>Select</button></td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    );
  }

  const selectedName = selectedVM ? vmField(selectedVM, 'Name', 'name', '') : '';
  const selectedID = selectedVM ? vmField(selectedVM, 'ID', 'id', '') : '';

  return (
    <div style={styles.card}>
      <div style={styles.headerRow}>
        <h2 style={styles.title}>Export {selectedName}</h2>
        <button style={styles.secondaryButton} onClick={() => setCurrentStep('discover')}>Back</button>
      </div>
      <div style={styles.selectedBox}>
        <strong>{selectedName}</strong> ({selectedID})
      </div>
      {error && <div style={styles.error}>{error}</div>}
      <form onSubmit={handleExportSubmit} style={styles.form}>
        <input style={styles.input} placeholder="Job name" value={exportOptions.jobName} onChange={(e) => setExportOptions({ ...exportOptions, jobName: e.target.value })} />
        <input style={styles.input} placeholder="Output directory" value={exportOptions.outputDir} onChange={(e) => setExportOptions({ ...exportOptions, outputDir: e.target.value })} required />
        <select style={styles.input} value={exportOptions.format} onChange={(e) => setExportOptions({ ...exportOptions, format: e.target.value })}>
          <option value="qcow2">qcow2</option>
          <option value="raw">raw</option>
        </select>
        <textarea
          style={{ ...styles.input, minHeight: '72px' }}
          placeholder="Container mounts (container:/path,container2:/path2)"
          value={exportOptions.mounts}
          onChange={(e) => setExportOptions({ ...exportOptions, mounts: e.target.value })}
          required
        />
        <label style={styles.checkbox}><input type="checkbox" checked={exportOptions.resolveContainers} onChange={(e) => setExportOptions({ ...exportOptions, resolveContainers: e.target.checked })} /> Resolve container names</label>
        <label style={styles.checkbox}><input type="checkbox" checked={exportOptions.enablePipeline} onChange={(e) => setExportOptions({ ...exportOptions, enablePipeline: e.target.checked })} /> Run h2kvm pipeline after export</label>
        <button style={styles.button} disabled={loading}>{loading ? 'Submitting...' : 'Submit Export Job'}</button>
      </form>
    </div>
  );
};

const styles: { [key: string]: React.CSSProperties } = {
  card: { backgroundColor: '#fff', borderRadius: '8px', padding: '24px', boxShadow: '0 1px 3px rgba(0,0,0,0.08)' },
  title: { margin: '0 0 8px', fontSize: '20px', fontWeight: 700 },
  subtitle: { margin: '0 0 16px', color: '#6b7280' },
  form: { display: 'grid', gap: '12px' },
  input: { padding: '10px 12px', border: '1px solid #e5e7eb', borderRadius: '6px', fontSize: '14px' },
  button: { padding: '12px', backgroundColor: '#f0583a', color: '#fff', border: 'none', borderRadius: '6px', fontWeight: 700, cursor: 'pointer' },
  secondaryButton: { padding: '8px 14px', backgroundColor: '#fff', border: '1px solid #d1d5db', borderRadius: '6px', cursor: 'pointer' },
  smallButton: { padding: '6px 10px', backgroundColor: '#f0583a', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer' },
  checkbox: { display: 'flex', alignItems: 'center', gap: '8px', fontSize: '14px' },
  error: { backgroundColor: '#fee2e2', color: '#991b1b', padding: '10px 12px', borderRadius: '6px', marginBottom: '12px' },
  headerRow: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' },
  tableWrap: { overflowX: 'auto', marginTop: '12px' },
  table: { width: '100%', borderCollapse: 'collapse', fontSize: '13px' },
  th: { textAlign: 'left', padding: '10px', borderBottom: '2px solid #e5e7eb', backgroundColor: '#f9fafb' },
  td: { padding: '10px', borderBottom: '1px solid #f3f4f6' },
  tdMono: { padding: '10px', borderBottom: '1px solid #f3f4f6', fontFamily: 'monospace', fontSize: '12px' },
  selectedBox: { padding: '12px', backgroundColor: '#ecfdf5', border: '1px solid #a7f3d0', borderRadius: '6px', marginBottom: '12px' },
};

export default NutanixExportWorkflow;
