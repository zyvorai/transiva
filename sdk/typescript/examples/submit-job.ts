// SPDX-License-Identifier: Apache-2.0

/**
 * Example: Submit a VM export job to HyperSDK
 */

import { HyperSDK, JobDefinition, ExportFormat } from '../src';

async function main() {
  // Initialize client
  const client = new HyperSDK('http://localhost:8080');

  try {
    // Login (if authentication is enabled)
    // await client.login('admin', 'password');

    // Check daemon health
    const health = await client.health();
    console.log(`✓ Daemon is ${health.status}`);

    // Get daemon status
    const status = await client.status();
    console.log(`✓ Version: ${status.version}`);
    console.log(`✓ Uptime: ${status.uptime}`);
    console.log(`✓ Total jobs: ${status.total_jobs}`);
    console.log(`✓ Running jobs: ${status.running_jobs}`);

    // Create job definition
    const jobDef: JobDefinition = {
      name: 'Export Ubuntu Server',
      vm_path: '/Datacenter/vm/ubuntu-server',
      output_dir: '/exports',
      vcenter: {
        server: 'vcenter.example.com',
        username: 'administrator@vsphere.local',
        password: 'your-password',
        insecure: true, // Skip TLS verification for development
      },
      format: ExportFormat.OVF,
      compress: true,
      thin: true,
    };

    // Submit job
    console.log('\nSubmitting job...');
    const jobId = await client.submitJob(jobDef);
    console.log(`✓ Job submitted successfully!`);
    console.log(`  Job ID: ${jobId}`);

    // Get job details
    const job = await client.getJob(jobId);
    console.log(`  Status: ${job.status}`);

    if (job.progress) {
      console.log(`  Progress: ${job.progress.percent_complete}%`);
      console.log(`  Phase: ${job.progress.phase}`);
    }

    console.log(`\nMonitor job progress with:`);
    console.log(`  ts-node examples/monitor-jobs.ts ${jobId}`);
  } catch (error) {
    console.error('✗ Error:', error);
    process.exit(1);
  }
}

main();
