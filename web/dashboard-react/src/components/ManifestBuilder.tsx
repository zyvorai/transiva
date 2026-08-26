// SPDX-License-Identifier: Apache-2.0

import React, { useState } from 'react';
import type { Manifest, ManifestPipeline } from '../types/metrics';

interface ManifestBuilderProps {
  apiUrl?: string;
  onSubmitSuccess?: (jobId: string) => void;
}

export const ManifestBuilder: React.FC<ManifestBuilderProps> = ({ apiUrl = '', onSubmitSuccess }) => {
  const [activeTab, setActiveTab] = useState<'form' | 'json'>('form');
  const [manifest, setManifest] = useState<Manifest>({
    version: '1.0',
    batch: false,
    pipeline: {
      load: {
        source_type: 'vmdk',
        source_path: '',
      },
      inspect: {
        enabled: true,
        detect_os: true,
      },
      fix: {
        fstab: {
          enabled: true,
          mode: 'stabilize-all',
        },
        grub: {
          enabled: true,
        },
        initramfs: {
          enabled: true,
          regenerate: true,
        },
        network: {
          enabled: true,
          fix_level: 'full',
        },
      },
      convert: {
        output_format: 'qcow2',
        compress: true,
      },
      validate: {
        enabled: true,
        boot_test: false,
      },
    },
  });

  const [validationErrors, setValidationErrors] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitSuccess, setSubmitSuccess] = useState<string | null>(null);

  const updatePipeline = <K extends keyof ManifestPipeline>(
    section: K,
    field: string,
    value: any
  ) => {
    setManifest((prev) => {
      if (!prev.pipeline) return prev;

      const pipeline = { ...prev.pipeline };
      const sectionData = { ...pipeline[section] } as any;

      if (field.includes('.')) {
        const [parent, child] = field.split('.');
        sectionData[parent] = { ...sectionData[parent], [child]: value };
      } else {
        sectionData[field] = value;
      }

      pipeline[section] = sectionData;
      return { ...prev, pipeline };
    });
  };

  const validateManifest = async () => {
    try {
      const response = await fetch(`${apiUrl}/api/workflow/manifest/validate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(manifest),
      });

      const result = await response.json();

      if (result.valid) {
        setValidationErrors([]);
        return true;
      } else {
        setValidationErrors(result.errors || ['Unknown validation error']);
        return false;
      }
    } catch (err) {
      setValidationErrors([err instanceof Error ? err.message : 'Validation failed']);
      return false;
    }
  };

  const handleSubmit = async () => {
    setSubmitting(true);
    setSubmitError(null);
    setSubmitSuccess(null);

    const isValid = await validateManifest();
    if (!isValid) {
      setSubmitting(false);
      return;
    }

    try {
      const response = await fetch(`${apiUrl}/api/workflow/manifest/submit`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(manifest),
      });

      if (!response.ok) {
        throw new Error('Failed to submit manifest');
      }

      const result = await response.json();
      setSubmitSuccess(`Manifest submitted! Job ID: ${result.job_id}`);

      if (onSubmitSuccess && result.job_id) {
        onSubmitSuccess(result.job_id);
      }
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Submit failed');
    } finally {
      setSubmitting(false);
    }
  };

  const downloadManifest = () => {
    const blob = new Blob([JSON.stringify(manifest, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `manifest-${Date.now()}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

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

  const inputStyle: React.CSSProperties = {
    width: '100%',
    padding: '6px 8px',
    fontSize: '10px',
    border: '1px solid #e0e0e0',
    borderRadius: '3px',
    fontFamily: 'inherit',
  };

  const labelStyle: React.CSSProperties = {
    display: 'block',
    fontSize: '9px',
    fontWeight: '600',
    marginBottom: '4px',
    color: '#333',
  };

  const sectionStyle: React.CSSProperties = {
    marginBottom: '16px',
    paddingBottom: '16px',
    borderBottom: '1px solid #e0e0e0',
  };

  const checkboxLabelStyle: React.CSSProperties = {
    display: 'flex',
    alignItems: 'center',
    gap: '6px',
    fontSize: '10px',
    cursor: 'pointer',
  };

  return (
    <div id="manifest-builder">
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
        <h2 style={{ margin: 0, fontSize: '14px', fontWeight: '600' }}>Manifest Builder</h2>

        <div style={{ display: 'flex', gap: '4px' }}>
          <button
            onClick={() => setActiveTab('form')}
            style={{
              padding: '4px 12px',
              fontSize: '10px',
              fontWeight: '600',
              borderRadius: '3px',
              border: '1px solid #222324',
              backgroundColor: activeTab === 'form' ? '#222324' : '#fff',
              color: activeTab === 'form' ? '#fff' : '#222324',
              cursor: 'pointer',
            }}
          >
            Form
          </button>
          <button
            onClick={() => setActiveTab('json')}
            style={{
              padding: '4px 12px',
              fontSize: '10px',
              fontWeight: '600',
              borderRadius: '3px',
              border: '1px solid #222324',
              backgroundColor: activeTab === 'json' ? '#222324' : '#fff',
              color: activeTab === 'json' ? '#fff' : '#222324',
              cursor: 'pointer',
            }}
          >
            JSON
          </button>
        </div>
      </div>

      {/* Validation Errors */}
      {validationErrors.length > 0 && (
        <div style={{ ...cardStyle, borderColor: '#dc3545', marginBottom: '12px' }}>
          <div style={{ ...headerStyle, marginBottom: '8px' }}>
            <div style={{ ...accentBarStyle, backgroundColor: '#dc3545' }} />
            <h3 style={{ margin: 0, fontSize: '11px', fontWeight: '600', color: '#dc3545' }}>
              Validation Errors
            </h3>
          </div>
          <ul style={{ margin: 0, paddingLeft: '20px', fontSize: '10px', color: '#dc3545' }}>
            {validationErrors.map((error, idx) => (
              <li key={idx}>{error}</li>
            ))}
          </ul>
        </div>
      )}

      {/* Submit Success */}
      {submitSuccess && (
        <div style={{ ...cardStyle, borderColor: '#28a745', marginBottom: '12px' }}>
          <div style={{ fontSize: '10px', color: '#28a745' }}>{submitSuccess}</div>
        </div>
      )}

      {/* Submit Error */}
      {submitError && (
        <div style={{ ...cardStyle, borderColor: '#dc3545', marginBottom: '12px' }}>
          <div style={{ fontSize: '10px', color: '#dc3545' }}>{submitError}</div>
        </div>
      )}

      {activeTab === 'form' ? (
        <div style={cardStyle}>
          {/* Source Configuration */}
          <div style={sectionStyle}>
            <div style={headerStyle}>
              <div style={accentBarStyle} />
              <h3 style={{ margin: 0, fontSize: '11px', fontWeight: '600' }}>1. Source Configuration</h3>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
              <div>
                <label style={labelStyle}>Source Type</label>
                <select
                  value={manifest.pipeline?.load.source_type || 'vmdk'}
                  onChange={(e) => updatePipeline('load', 'source_type', e.target.value)}
                  style={inputStyle}
                >
                  <option value="vmdk">VMDK</option>
                  <option value="ova">OVA</option>
                  <option value="ovf">OVF</option>
                  <option value="vhd">VHD</option>
                  <option value="vhdx">VHDX</option>
                  <option value="raw">RAW</option>
                </select>
              </div>

              <div>
                <label style={labelStyle}>Source Path</label>
                <input
                  type="text"
                  value={manifest.pipeline?.load.source_path || ''}
                  onChange={(e) => updatePipeline('load', 'source_path', e.target.value)}
                  placeholder="/path/to/disk.vmdk"
                  style={inputStyle}
                />
              </div>
            </div>
          </div>

          {/* Pipeline Stages */}
          <div style={sectionStyle}>
            <div style={headerStyle}>
              <div style={accentBarStyle} />
              <h3 style={{ margin: 0, fontSize: '11px', fontWeight: '600' }}>2. Pipeline Stages</h3>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <label style={checkboxLabelStyle}>
                <input
                  type="checkbox"
                  checked={manifest.pipeline?.inspect.enabled || false}
                  onChange={(e) => updatePipeline('inspect', 'enabled', e.target.checked)}
                />
                Enable INSPECT stage (detect OS and analyze disk)
              </label>

              <label style={checkboxLabelStyle}>
                <input
                  type="checkbox"
                  checked={manifest.pipeline?.fix.fstab.enabled || false}
                  onChange={(e) => updatePipeline('fix', 'fstab.enabled', e.target.checked)}
                />
                Enable FIX stage (fstab, grub, initramfs, network)
              </label>

              <label style={checkboxLabelStyle}>
                <input
                  type="checkbox"
                  checked={manifest.pipeline?.validate.enabled || false}
                  onChange={(e) => updatePipeline('validate', 'enabled', e.target.checked)}
                />
                Enable VALIDATE stage (verify converted image)
              </label>
            </div>
          </div>

          {/* Output Configuration */}
          <div style={sectionStyle}>
            <div style={headerStyle}>
              <div style={accentBarStyle} />
              <h3 style={{ margin: 0, fontSize: '11px', fontWeight: '600' }}>3. Output Configuration</h3>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '12px' }}>
              <div>
                <label style={labelStyle}>Output Format</label>
                <select
                  value={manifest.pipeline?.convert.output_format || 'qcow2'}
                  onChange={(e) => updatePipeline('convert', 'output_format', e.target.value)}
                  style={inputStyle}
                >
                  <option value="qcow2">QCOW2</option>
                  <option value="raw">RAW</option>
                  <option value="vmdk">VMDK</option>
                  <option value="vdi">VDI</option>
                </select>
              </div>

              <div>
                <label style={labelStyle}>Output Path (optional)</label>
                <input
                  type="text"
                  value={manifest.pipeline?.convert.output_path || ''}
                  onChange={(e) => updatePipeline('convert', 'output_path', e.target.value)}
                  placeholder="/path/to/output/"
                  style={inputStyle}
                />
              </div>
            </div>

            <label style={checkboxLabelStyle}>
              <input
                type="checkbox"
                checked={manifest.pipeline?.convert.compress || false}
                onChange={(e) => updatePipeline('convert', 'compress', e.target.checked)}
              />
              Enable compression
            </label>
          </div>

          {/* Actions */}
          <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
            <button
              onClick={validateManifest}
              style={{
                padding: '8px 16px',
                fontSize: '10px',
                fontWeight: '600',
                borderRadius: '3px',
                border: '1px solid #222324',
                backgroundColor: '#fff',
                cursor: 'pointer',
                transition: 'all 0.2s',
              }}
            >
              Validate
            </button>

            <button
              onClick={handleSubmit}
              disabled={submitting}
              style={{
                padding: '8px 16px',
                fontSize: '10px',
                fontWeight: '600',
                borderRadius: '3px',
                border: '1px solid #222324',
                backgroundColor: '#f0583a',
                color: '#fff',
                cursor: submitting ? 'not-allowed' : 'pointer',
                opacity: submitting ? 0.6 : 1,
                transition: 'all 0.2s',
              }}
            >
              {submitting ? 'Submitting...' : 'Submit to Workflow'}
            </button>

            <button
              onClick={downloadManifest}
              style={{
                padding: '8px 16px',
                fontSize: '10px',
                fontWeight: '600',
                borderRadius: '3px',
                border: '1px solid #222324',
                backgroundColor: '#fff',
                cursor: 'pointer',
                transition: 'all 0.2s',
              }}
            >
              Download JSON
            </button>
          </div>
        </div>
      ) : (
        <div style={cardStyle}>
          <div style={headerStyle}>
            <div style={accentBarStyle} />
            <h3 style={{ margin: 0, fontSize: '11px', fontWeight: '600' }}>Manifest JSON</h3>
          </div>

          <textarea
            value={JSON.stringify(manifest, null, 2)}
            onChange={(e) => {
              try {
                setManifest(JSON.parse(e.target.value));
                setValidationErrors([]);
              } catch (err) {
                setValidationErrors(['Invalid JSON format']);
              }
            }}
            style={{
              width: '100%',
              minHeight: '400px',
              padding: '8px',
              fontSize: '10px',
              fontFamily: 'monospace',
              border: '1px solid #e0e0e0',
              borderRadius: '3px',
              resize: 'vertical',
            }}
          />

          <div style={{ display: 'flex', gap: '8px', marginTop: '12px' }}>
            <button
              onClick={validateManifest}
              style={{
                padding: '8px 16px',
                fontSize: '10px',
                fontWeight: '600',
                borderRadius: '3px',
                border: '1px solid #222324',
                backgroundColor: '#fff',
                cursor: 'pointer',
              }}
            >
              Validate
            </button>

            <button
              onClick={handleSubmit}
              disabled={submitting}
              style={{
                padding: '8px 16px',
                fontSize: '10px',
                fontWeight: '600',
                borderRadius: '3px',
                border: '1px solid #222324',
                backgroundColor: '#f0583a',
                color: '#fff',
                cursor: submitting ? 'not-allowed' : 'pointer',
                opacity: submitting ? 0.6 : 1,
              }}
            >
              {submitting ? 'Submitting...' : 'Submit to Workflow'}
            </button>

            <button
              onClick={downloadManifest}
              style={{
                padding: '8px 16px',
                fontSize: '10px',
                fontWeight: '600',
                borderRadius: '3px',
                border: '1px solid #222324',
                backgroundColor: '#fff',
                cursor: 'pointer',
              }}
            >
              Download JSON
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
