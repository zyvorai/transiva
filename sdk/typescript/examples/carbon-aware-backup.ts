#!/usr/bin/env node
// SPDX-License-Identifier: Apache-2.0

/**
 * Carbon-Aware Backup Example
 *
 * Demonstrates how to use HyperSDK's carbon-aware features to reduce the
 * carbon footprint of VM backups by 30-50%.
 *
 * Features:
 * - Check grid carbon status before backup
 * - Estimate potential carbon savings
 * - Submit carbon-aware backup jobs
 * - Generate carbon reports for ESG compliance
 * - List available carbon zones
 */

import { HyperSDK, JobDefinition } from '../src';

// Configuration
const DAEMON_URL = 'http://localhost:8080';
const CARBON_ZONE = 'US-CAL-CISO'; // California grid
const THRESHOLD = 200.0; // gCO2/kWh - good/moderate boundary

async function main() {
  // Initialize client
  const client = new HyperSDK(DAEMON_URL);

  console.log('🌿 Carbon-Aware Backup Example');
  console.log('='.repeat(60));
  console.log();

  // Example 1: Check grid status before backup
  console.log('1️⃣  Checking Grid Carbon Status');
  console.log('-'.repeat(60));
  const status = await client.getCarbonStatus(CARBON_ZONE, THRESHOLD);

  console.log(`Zone: ${status.zone}`);
  console.log(`Current Intensity: ${status.current_intensity.toFixed(1)} gCO2/kWh`);
  console.log(`Quality: ${status.quality.toUpperCase()}`);
  console.log(`Renewable Energy: ${status.renewable_percent.toFixed(1)}%`);
  console.log(`Optimal for Backup: ${status.optimal_for_backup ? '✓ YES' : '✗ NO'}`);
  console.log(`Reasoning: ${status.reasoning}`);

  if (status.next_optimal_time) {
    const nextOptimal = new Date(status.next_optimal_time);
    const delay = Math.floor((nextOptimal.getTime() - Date.now()) / 60000);
    console.log(
      `Next Optimal Time: ${nextOptimal.toLocaleTimeString()} (in ${delay} minutes)`
    );
  }

  console.log();

  // Example 2: View 4-hour forecast
  console.log('2️⃣  4-Hour Forecast');
  console.log('-'.repeat(60));
  console.log(`${'Time'.padEnd(10)} ${'Intensity'.padEnd(15)} ${'Quality'.padEnd(12)}`);
  console.log('-'.repeat(60));
  for (const f of status.forecast_next_4h.slice(0, 4)) {
    // Show first 4 hours
    const time = new Date(f.time).toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
    });
    const intensity = `${f.intensity_gco2_kwh.toFixed(1)} gCO2/kWh`;
    console.log(`${time.padEnd(10)} ${intensity.padStart(15)}   ${f.quality.padEnd(12)}`);
  }
  console.log();

  // Example 3: List available zones
  console.log('3️⃣  Available Carbon Zones');
  console.log('-'.repeat(60));
  const zones = await client.listCarbonZones();

  // Group by region
  const regions: { [key: string]: typeof zones } = {};
  for (const zone of zones) {
    if (!regions[zone.region]) {
      regions[zone.region] = [];
    }
    regions[zone.region].push(zone);
  }

  for (const [region, regionZones] of Object.entries(regions)) {
    console.log(`\n📍 ${region}`);
    for (const zone of regionZones) {
      const id = zone.id.padEnd(15);
      const name = zone.name.padEnd(30);
      const intensity = `${zone.typical_intensity.toFixed(0)} gCO2/kWh`.padStart(5);
      console.log(`  ${id} ${name} ${intensity}`);
    }
  }
  console.log();

  // Example 4: Estimate carbon savings
  console.log('4️⃣  Estimating Carbon Savings');
  console.log('-'.repeat(60));
  const DATA_SIZE_GB = 500.0;
  const DURATION_HOURS = 2.0;

  const estimate = await client.estimateCarbonSavings(
    CARBON_ZONE,
    DATA_SIZE_GB,
    DURATION_HOURS
  );

  console.log(`Data Size: ${DATA_SIZE_GB} GB`);
  console.log(`Duration: ${DURATION_HOURS} hours`);
  console.log();
  console.log('Run Now:');
  console.log(`  Intensity: ${estimate.current_intensity_gco2_kwh.toFixed(1)} gCO2/kWh`);
  console.log(`  Emissions: ${estimate.current_emissions_kg_co2.toFixed(3)} kg CO2`);
  console.log();
  console.log('Run at Best Time:');
  console.log(`  Intensity: ${estimate.best_intensity_gco2_kwh.toFixed(1)} gCO2/kWh`);
  console.log(`  Emissions: ${estimate.best_emissions_kg_co2.toFixed(3)} kg CO2`);
  if (estimate.best_time) {
    const bestTime = new Date(estimate.best_time);
    console.log(`  Best Time: ${bestTime.toLocaleTimeString()}`);
  }
  if (estimate.delay_minutes !== undefined) {
    console.log(`  Delay: ${estimate.delay_minutes.toFixed(0)} minutes`);
  }
  console.log();
  console.log(
    `💰 Savings: ${estimate.savings_kg_co2.toFixed(3)} kg CO2 (${estimate.savings_percent.toFixed(1)}% reduction)`
  );
  console.log(`💡 ${estimate.recommendation}`);
  console.log();

  // Example 5: Submit carbon-aware job
  console.log('5️⃣  Submitting Carbon-Aware Backup Job');
  console.log('-'.repeat(60));

  const jobDef: JobDefinition = {
    vm_path: '/datacenter/vm/prod-db',
    output_dir: '/backups',
    name: 'nightly-backup-carbon-aware',
  };

  // Decision logic based on grid status
  let jobId: string;
  if (status.optimal_for_backup) {
    console.log('✅ Grid is clean - submitting backup now!');
    jobId = await client.submitCarbonAwareJob(jobDef, CARBON_ZONE, THRESHOLD, 4.0);
    console.log(`Job ID: ${jobId}`);
  } else {
    if (estimate.savings_percent > 30) {
      // If savings > 30%
      console.log(
        `⏰ Grid is dirty - delaying backup for ${estimate.delay_minutes?.toFixed(0)} min`
      );
      console.log(`   Expected savings: ${estimate.savings_percent.toFixed(1)}%`);
      jobId = await client.submitCarbonAwareJob(jobDef, CARBON_ZONE, THRESHOLD, 4.0);
      console.log(`Job ID (will be delayed): ${jobId}`);
    } else {
      console.log('⚠️  Savings < 30% - running backup now despite dirty grid');
      jobId = await client.submitJob(jobDef);
      console.log(`Job ID: ${jobId}`);
    }
  }
  console.log();

  // Example 6: Generate carbon report (simulated for completed job)
  console.log('6️⃣  Generating Carbon Report (Example)');
  console.log('-'.repeat(60));

  // Simulate a completed job
  const endTime = new Date();
  const startTime = new Date(endTime.getTime() - 2 * 60 * 60 * 1000); // 2 hours ago

  try {
    const report = await client.getCarbonReport(
      'job-example-123',
      startTime.toISOString(),
      endTime.toISOString(),
      DATA_SIZE_GB,
      CARBON_ZONE
    );

    console.log(`Job ID: ${report.operation_id}`);
    console.log(`Duration: ${report.duration_hours.toFixed(1)} hours`);
    console.log(`Data Size: ${report.data_size_gb.toFixed(1)} GB`);
    console.log();
    console.log('⚡ Energy & Emissions:');
    console.log(`  Energy Used: ${report.energy_kwh.toFixed(3)} kWh`);
    console.log(
      `  Carbon Intensity: ${report.carbon_intensity_gco2_kwh.toFixed(1)} gCO2/kWh`
    );
    console.log(`  Emissions: ${report.carbon_emissions_kg_co2.toFixed(3)} kg CO2`);
    console.log(`  Renewable Energy: ${report.renewable_percent.toFixed(1)}%`);
    console.log();
    console.log('💰 Savings:');
    console.log(`  vs Worst Case: ${report.savings_vs_worst_kg_co2.toFixed(3)} kg CO2`);
    console.log(`  Equivalent: ${report.equivalent}`);
    console.log();
  } catch (error: any) {
    console.log('Note: Report generation requires completed job (simulation)');
    console.log(`Error: ${error.message}`);
    console.log();
  }

  // Example 7: Decision workflow
  console.log('7️⃣  Complete Decision Workflow');
  console.log('-'.repeat(60));
  printDecisionWorkflow(status, estimate);
  console.log();

  // Example 8: Best practices
  console.log('8️⃣  Best Practices');
  console.log('-'.repeat(60));
  printBestPractices();
  console.log();

  console.log('✅ Example completed!');
  console.log();
  console.log('Next Steps:');
  console.log('  • Integrate into your backup scripts');
  console.log('  • Set up cron jobs with carbon checking');
  console.log('  • Generate carbon reports for ESG compliance');
  console.log('  • Monitor carbon savings over time');
}

function printDecisionWorkflow(status: any, estimate: any) {
  console.log('Decision Tree:');
  console.log();
  console.log('1. Check grid status');
  console.log(
    `   ├─ Current: ${status.current_intensity.toFixed(0)} gCO2/kWh (${status.quality})`
  );
  console.log(`   └─ Threshold: ${THRESHOLD.toFixed(0)} gCO2/kWh`);
  console.log();

  if (status.optimal_for_backup) {
    console.log('2. ✅ Grid is CLEAN → Submit backup immediately');
  } else {
    console.log('2. ❌ Grid is DIRTY → Check savings estimate');
    console.log(`   ├─ Potential savings: ${estimate.savings_percent.toFixed(1)}%`);
    if (estimate.savings_percent > 30) {
      console.log('   └─ Savings > 30% → DELAY backup');
      if (estimate.delay_minutes) {
        console.log(`      └─ Delay: ${estimate.delay_minutes.toFixed(0)} minutes`);
      }
    } else {
      console.log('   └─ Savings < 30% → RUN backup now');
    }
  }
  console.log();
  console.log('3. Submit job with carbon_aware metadata');
  console.log('   └─ Scheduler will handle delay automatically');
}

function printBestPractices() {
  const practices = [
    'Check grid status before each backup',
    'Set appropriate threshold (150-250 gCO2/kWh)',
    'Allow 2-4 hour delay window for flexibility',
    'Generate carbon reports for ESG compliance',
    'Choose cleanest datacenter zone when possible',
    'Monitor carbon savings over time',
    'Schedule backups during typical clean periods (midday for solar)',
    'Use lower threshold for critical sustainability goals',
  ];

  practices.forEach((practice, i) => {
    console.log(`  ${i + 1}. ${practice}`);
  });
}

async function printZoneComparison() {
  console.log('Zone Comparison Example:');
  console.log();

  const client = new HyperSDK(DAEMON_URL);
  const zonesToCheck = ['US-CAL-CISO', 'DE', 'SE'];

  console.log(`${'Zone'.padEnd(15)} ${'Intensity'.padEnd(15)} ${'Quality'.padEnd(12)} ${'Renewable'.padEnd(12)}`);
  console.log('-'.repeat(60));

  for (const zoneId of zonesToCheck) {
    try {
      const status = await client.getCarbonStatus(zoneId);
      const intensity = `${status.current_intensity.toFixed(1)} gCO2/kWh`;
      const renewable = `${status.renewable_percent.toFixed(1)}%`;
      console.log(
        `${zoneId.padEnd(15)} ${intensity.padStart(15)}   ${status.quality.padEnd(12)} ${renewable.padStart(12)}`
      );
    } catch (error: any) {
      console.log(`${zoneId.padEnd(15)} Error: ${error.message}`);
    }
  }
}

main().catch((error) => {
  console.error('\n❌ Error:', error.message);
  if (error.stack) {
    console.error(error.stack);
  }
  process.exit(1);
});
