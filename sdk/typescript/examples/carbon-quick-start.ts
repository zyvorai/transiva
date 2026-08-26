#!/usr/bin/env node
// SPDX-License-Identifier: Apache-2.0

/**
 * Carbon-Aware Quick Start
 *
 * Simple example showing the essentials of carbon-aware backups.
 */

import { HyperSDK, JobDefinition } from '../src';

async function main() {
  // Initialize client
  const client = new HyperSDK('http://localhost:8080');

  console.log('🌿 Carbon-Aware Backup - Quick Start\n');

  // 1. Check if grid is clean
  const status = await client.getCarbonStatus('US-CAL-CISO', 200);
  console.log(`Grid Status: ${status.quality.toUpperCase()}`);
  console.log(`Carbon Intensity: ${status.current_intensity.toFixed(0)} gCO2/kWh`);
  console.log(`Optimal for Backup: ${status.optimal_for_backup ? '✓ Yes' : '✗ No'}\n`);

  // 2. Estimate savings
  const estimate = await client.estimateCarbonSavings('US-CAL-CISO', 500, 2);
  console.log(`Potential Savings: ${estimate.savings_percent.toFixed(1)}%`);
  console.log(`Recommendation: ${estimate.recommendation}\n`);

  // 3. Submit carbon-aware backup
  const jobDef: JobDefinition = {
    vm_path: '/datacenter/vm/prod-db',
    output_dir: '/backups',
  };

  let jobId: string;
  if (status.optimal_for_backup) {
    console.log('✅ Submitting backup now (grid is clean)');
    jobId = await client.submitCarbonAwareJob(jobDef, 'US-CAL-CISO', 200);
  } else {
    console.log('⏰ Grid is dirty - job will be delayed for cleaner period');
    jobId = await client.submitCarbonAwareJob(jobDef, 'US-CAL-CISO', 200, 4);
  }

  console.log(`Job ID: ${jobId}`);
  console.log('\n🎉 Carbon-aware backup scheduled!');
}

main().catch((error) => {
  console.error('Error:', error.message);
  process.exit(1);
});
