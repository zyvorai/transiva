#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0

"""
Carbon-Aware Quick Start

Simple example showing the essentials of carbon-aware backups.
"""

from transiva import HyperSDK, JobDefinition

# Initialize client
client = HyperSDK("http://localhost:8080")

print("🌿 Carbon-Aware Backup - Quick Start\n")

# 1. Check if grid is clean
status = client.get_carbon_status(zone="US-CAL-CISO", threshold=200)
print(f"Grid Status: {status.quality.upper()}")
print(f"Carbon Intensity: {status.current_intensity:.0f} gCO2/kWh")
print(f"Optimal for Backup: {'✓ Yes' if status.optimal_for_backup else '✗ No'}\n")

# 2. Estimate savings
estimate = client.estimate_carbon_savings(
    zone="US-CAL-CISO",
    data_size_gb=500,
    duration_hours=2
)
print(f"Potential Savings: {estimate.savings_percent:.1f}%")
print(f"Recommendation: {estimate.recommendation}\n")

# 3. Submit carbon-aware backup
job_def = JobDefinition(
    vm_path="/datacenter/vm/prod-db",
    output_dir="/backups"
)

if status.optimal_for_backup:
    print("✅ Submitting backup now (grid is clean)")
    job_id = client.submit_carbon_aware_job(job_def, max_intensity=200)
else:
    print("⏰ Grid is dirty - job will be delayed for cleaner period")
    job_id = client.submit_carbon_aware_job(job_def, max_intensity=200, max_delay_hours=4)

print(f"Job ID: {job_id}")
print("\n🎉 Carbon-aware backup scheduled!")
