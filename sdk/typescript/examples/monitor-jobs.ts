// SPDX-License-Identifier: Apache-2.0

/**
 * Example: Monitor job progress in real-time
 */

import { HyperSDK, JobStatus } from '../src';

function formatBytes(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = bytes;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex++;
  }

  return `${value.toFixed(2)} ${units[unitIndex]}`;
}

async function monitorJob(client: HyperSDK, jobId: string): Promise<number> {
  console.log(`Monitoring job: ${jobId}\n`);

  while (true) {
    try {
      const job = await client.getJob(jobId);

      // Clear line and print status
      process.stdout.write('\r\x1b[K'); // Clear current line
      process.stdout.write(`Status: ${job.status}`);

      if (job.progress) {
        const progress = job.progress;
        process.stdout.write(` | Progress: ${progress.percent_complete.toFixed(1)}%`);
        process.stdout.write(` | Phase: ${progress.phase}`);

        if (progress.current_file) {
          process.stdout.write(` | File: ${progress.current_file}`);
        }

        if (progress.bytes_downloaded && progress.total_bytes) {
          const downloaded = formatBytes(progress.bytes_downloaded);
          const total = formatBytes(progress.total_bytes);
          process.stdout.write(` | ${downloaded}/${total}`);
        }

        if (progress.estimated_remaining) {
          process.stdout.write(` | ETA: ${progress.estimated_remaining}`);
        }
      }

      // Check if job is complete
      if (
        job.status === JobStatus.COMPLETED ||
        job.status === JobStatus.FAILED ||
        job.status === JobStatus.CANCELLED
      ) {
        console.log(); // New line
        break;
      }

      await new Promise((resolve) => setTimeout(resolve, 2000)); // Poll every 2 seconds
    } catch (error: any) {
      console.error('\nError:', error.message);
      return 1;
    }
  }

  // Get final job state
  const job = await client.getJob(jobId);

  // Print final result
  console.log('='.repeat(80));
  if (job.status === JobStatus.COMPLETED) {
    console.log('✓ Job completed successfully!');
    if (job.result) {
      const result = job.result;
      console.log(`  VM Name: ${result.vm_name}`);
      console.log(`  Output Directory: ${result.output_dir}`);
      console.log(`  Total Size: ${formatBytes(result.total_size)}`);
      console.log(`  Duration: ${(result.duration / 1e9).toFixed(2)} seconds`);
      console.log(`  Export Method: ${result.export_method}`);
      console.log(`  Files:`);
      for (const file of result.files) {
        console.log(`    - ${file}`);
      }
    }
  } else if (job.status === JobStatus.FAILED) {
    console.log('✗ Job failed!');
    if (job.error) {
      console.log(`  Error: ${job.error}`);
    }
  } else if (job.status === JobStatus.CANCELLED) {
    console.log('⊘ Job was cancelled');
  }

  return 0;
}

async function monitorAllJobs(client: HyperSDK): Promise<number> {
  console.log('Monitoring all jobs...\n');

  while (true) {
    try {
      // Get all jobs
      const jobs = await client.listJobs(true);

      // Filter running jobs
      const runningJobs = jobs.filter((j) => j.status === JobStatus.RUNNING);

      if (runningJobs.length === 0) {
        console.log('No running jobs.');
        break;
      }

      // Clear screen
      process.stdout.write('\x1b[2J\x1b[H');

      // Print header
      console.log('='.repeat(80));
      console.log(`Running Jobs: ${runningJobs.length}`);
      console.log('='.repeat(80));
      console.log();

      // Print each job
      for (const job of runningJobs) {
        const jobId = job.definition.id || 'Unknown';
        const name = job.definition.name || job.definition.vm_path;
        console.log(`Job: ${jobId} - ${name}`);

        if (job.progress) {
          const progress = job.progress;
          const barLength = 40;
          const filled = Math.floor((barLength * progress.percent_complete) / 100);
          const bar = '█'.repeat(filled) + '░'.repeat(barLength - filled);
          console.log(`  [${bar}] ${progress.percent_complete.toFixed(1)}%`);
          console.log(`  Phase: ${progress.phase}`);

          if (progress.estimated_remaining) {
            console.log(`  ETA: ${progress.estimated_remaining}`);
          }
        }

        console.log();
      }

      await new Promise((resolve) => setTimeout(resolve, 3000)); // Refresh every 3 seconds
    } catch (error: any) {
      console.error('\nError:', error.message);
      return 1;
    }
  }

  return 0;
}

async function main() {
  const args = process.argv.slice(2);
  const jobId = args[0];
  const mode = jobId ? 'single' : 'all';

  // Initialize client
  const client = new HyperSDK('http://localhost:8080');

  try {
    // Login (if authentication is enabled)
    // await client.login('admin', 'password');

    if (mode === 'single') {
      return await monitorJob(client, jobId);
    } else {
      return await monitorAllJobs(client);
    }
  } catch (error: any) {
    console.error('Error:', error.message);
    return 1;
  }
}

main().then((exitCode) => process.exit(exitCode));
