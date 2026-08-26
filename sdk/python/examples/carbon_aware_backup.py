#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0

"""
Carbon-Aware Backup Example

Demonstrates how to use HyperSDK's carbon-aware features to reduce the
carbon footprint of VM backups by 30-50%.

Features:
- Check grid carbon status before backup
- Estimate potential carbon savings
- Submit carbon-aware backup jobs
- Generate carbon reports for ESG compliance
- List available carbon zones
"""

from datetime import datetime, timedelta
from transiva import HyperSDK, JobDefinition

# Configuration
DAEMON_URL = "http://localhost:8080"
CARBON_ZONE = "US-CAL-CISO"  # California grid
THRESHOLD = 200.0  # gCO2/kWh - good/moderate boundary


def main():
    """Main example workflow."""
    # Initialize client
    client = HyperSDK(DAEMON_URL)

    print("🌿 Carbon-Aware Backup Example")
    print("=" * 60)
    print()

    # Example 1: Check grid status before backup
    print("1️⃣  Checking Grid Carbon Status")
    print("-" * 60)
    status = client.get_carbon_status(zone=CARBON_ZONE, threshold=THRESHOLD)

    print(f"Zone: {status.zone}")
    print(f"Current Intensity: {status.current_intensity:.1f} gCO2/kWh")
    print(f"Quality: {status.quality.upper()}")
    print(f"Renewable Energy: {status.renewable_percent:.1f}%")
    print(f"Optimal for Backup: {'✓ YES' if status.optimal_for_backup else '✗ NO'}")
    print(f"Reasoning: {status.reasoning}")

    if status.next_optimal_time:
        delay = status.next_optimal_time - datetime.now()
        print(f"Next Optimal Time: {status.next_optimal_time.strftime('%H:%M:%S')} (in {delay.seconds // 60} minutes)")

    print()

    # Example 2: View 4-hour forecast
    print("2️⃣  4-Hour Forecast")
    print("-" * 60)
    print(f"{'Time':<10} {'Intensity':<15} {'Quality':<12}")
    print("-" * 60)
    for f in status.forecast[:4]:  # Show first 4 hours
        print(f"{f.time.strftime('%H:%M'):<10} {f.intensity_gco2_kwh:>8.1f} gCO2/kWh   {f.quality:<12}")
    print()

    # Example 3: List available zones
    print("3️⃣  Available Carbon Zones")
    print("-" * 60)
    zones = client.list_carbon_zones()

    # Group by region
    regions = {}
    for zone in zones:
        if zone.region not in regions:
            regions[zone.region] = []
        regions[zone.region].append(zone)

    for region, region_zones in regions.items():
        print(f"\n📍 {region}")
        for zone in region_zones:
            print(f"  {zone.id:<15} {zone.name:<30} {zone.typical_intensity:>5.0f} gCO2/kWh")
    print()

    # Example 4: Estimate carbon savings
    print("4️⃣  Estimating Carbon Savings")
    print("-" * 60)
    DATA_SIZE_GB = 500.0
    DURATION_HOURS = 2.0

    estimate = client.estimate_carbon_savings(
        zone=CARBON_ZONE,
        data_size_gb=DATA_SIZE_GB,
        duration_hours=DURATION_HOURS
    )

    print(f"Data Size: {DATA_SIZE_GB} GB")
    print(f"Duration: {DURATION_HOURS} hours")
    print()
    print(f"Run Now:")
    print(f"  Intensity: {estimate.current_intensity_gco2_kwh:.1f} gCO2/kWh")
    print(f"  Emissions: {estimate.current_emissions_kg_co2:.3f} kg CO2")
    print()
    print(f"Run at Best Time:")
    print(f"  Intensity: {estimate.best_intensity_gco2_kwh:.1f} gCO2/kWh")
    print(f"  Emissions: {estimate.best_emissions_kg_co2:.3f} kg CO2")
    if estimate.best_time:
        print(f"  Best Time: {estimate.best_time.strftime('%H:%M:%S')}")
    if estimate.delay_minutes:
        print(f"  Delay: {estimate.delay_minutes:.0f} minutes")
    print()
    print(f"💰 Savings: {estimate.savings_kg_co2:.3f} kg CO2 ({estimate.savings_percent:.1f}% reduction)")
    print(f"💡 {estimate.recommendation}")
    print()

    # Example 5: Submit carbon-aware job
    print("5️⃣  Submitting Carbon-Aware Backup Job")
    print("-" * 60)

    job_def = JobDefinition(
        vm_path="/datacenter/vm/prod-db",
        output_dir="/backups",
        name="nightly-backup-carbon-aware"
    )

    # Decision logic based on grid status
    if status.optimal_for_backup:
        print("✅ Grid is clean - submitting backup now!")
        job_id = client.submit_carbon_aware_job(
            job_def,
            carbon_zone=CARBON_ZONE,
            max_intensity=THRESHOLD,
            max_delay_hours=4.0
        )
        print(f"Job ID: {job_id}")
    else:
        if estimate.savings_percent > 30:  # If savings > 30%
            print(f"⏰ Grid is dirty - delaying backup for {estimate.delay_minutes:.0f} min")
            print(f"   Expected savings: {estimate.savings_percent:.1f}%")
            job_id = client.submit_carbon_aware_job(
                job_def,
                carbon_zone=CARBON_ZONE,
                max_intensity=THRESHOLD,
                max_delay_hours=4.0
            )
            print(f"Job ID (will be delayed): {job_id}")
        else:
            print("⚠️  Savings < 30% - running backup now despite dirty grid")
            job_id = client.submit_job(job_def)
            print(f"Job ID: {job_id}")
    print()

    # Example 6: Generate carbon report (simulated for completed job)
    print("6️⃣  Generating Carbon Report (Example)")
    print("-" * 60)

    # Simulate a completed job
    start_time = datetime.now() - timedelta(hours=2)
    end_time = datetime.now()

    try:
        report = client.get_carbon_report(
            job_id="job-example-123",
            start_time=start_time,
            end_time=end_time,
            data_size_gb=DATA_SIZE_GB,
            zone=CARBON_ZONE
        )

        print(f"Job ID: {report.operation_id}")
        print(f"Duration: {report.duration_hours:.1f} hours")
        print(f"Data Size: {report.data_size_gb:.1f} GB")
        print()
        print(f"⚡ Energy & Emissions:")
        print(f"  Energy Used: {report.energy_kwh:.3f} kWh")
        print(f"  Carbon Intensity: {report.carbon_intensity_gco2_kwh:.1f} gCO2/kWh")
        print(f"  Emissions: {report.carbon_emissions_kg_co2:.3f} kg CO2")
        print(f"  Renewable Energy: {report.renewable_percent:.1f}%")
        print()
        print(f"💰 Savings:")
        print(f"  vs Worst Case: {report.savings_vs_worst_kg_co2:.3f} kg CO2")
        print(f"  Equivalent: {report.equivalent}")
        print()
    except Exception as e:
        print(f"Note: Report generation requires completed job (simulation)")
        print(f"Error: {e}")
        print()

    # Example 7: Decision workflow
    print("7️⃣  Complete Decision Workflow")
    print("-" * 60)
    print_decision_workflow(status, estimate)
    print()

    # Example 8: Best practices
    print("8️⃣  Best Practices")
    print("-" * 60)
    print_best_practices()
    print()

    print("✅ Example completed!")
    print()
    print("Next Steps:")
    print("  • Integrate into your backup scripts")
    print("  • Set up cron jobs with carbon checking")
    print("  • Generate carbon reports for ESG compliance")
    print("  • Monitor carbon savings over time")


def print_decision_workflow(status, estimate):
    """Print decision workflow example."""
    print("Decision Tree:")
    print()
    print("1. Check grid status")
    print(f"   ├─ Current: {status.current_intensity:.0f} gCO2/kWh ({status.quality})")
    print(f"   └─ Threshold: {THRESHOLD:.0f} gCO2/kWh")
    print()

    if status.optimal_for_backup:
        print("2. ✅ Grid is CLEAN → Submit backup immediately")
    else:
        print("2. ❌ Grid is DIRTY → Check savings estimate")
        print(f"   ├─ Potential savings: {estimate.savings_percent:.1f}%")
        if estimate.savings_percent > 30:
            print("   └─ Savings > 30% → DELAY backup")
            if estimate.delay_minutes:
                print(f"      └─ Delay: {estimate.delay_minutes:.0f} minutes")
        else:
            print("   └─ Savings < 30% → RUN backup now")
    print()
    print("3. Submit job with carbon_aware metadata")
    print("   └─ Scheduler will handle delay automatically")


def print_best_practices():
    """Print best practices."""
    practices = [
        "Check grid status before each backup",
        "Set appropriate threshold (150-250 gCO2/kWh)",
        "Allow 2-4 hour delay window for flexibility",
        "Generate carbon reports for ESG compliance",
        "Choose cleanest datacenter zone when possible",
        "Monitor carbon savings over time",
        "Schedule backups during typical clean periods (midday for solar)",
        "Use lower threshold for critical sustainability goals",
    ]

    for i, practice in enumerate(practices, 1):
        print(f"  {i}. {practice}")


def print_zone_comparison():
    """Compare carbon intensity across zones."""
    print("Zone Comparison Example:")
    print()

    client = HyperSDK(DAEMON_URL)
    zones_to_check = ["US-CAL-CISO", "DE", "SE"]

    print(f"{'Zone':<15} {'Intensity':<15} {'Quality':<12} {'Renewable':<12}")
    print("-" * 60)

    for zone_id in zones_to_check:
        try:
            status = client.get_carbon_status(zone=zone_id)
            print(f"{zone_id:<15} {status.current_intensity:>8.1f} gCO2/kWh   {status.quality:<12} {status.renewable_percent:>6.1f}%")
        except Exception as e:
            print(f"{zone_id:<15} Error: {e}")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n\nInterrupted by user")
    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()
