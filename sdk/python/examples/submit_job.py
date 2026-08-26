#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0

"""Example: Submit a VM export job to HyperSDK."""

from transiva import HyperSDK, JobDefinition, VCenterConfig, ExportFormat

def main():
    # Initialize client
    client = HyperSDK("http://localhost:8080")

    try:
        # Login (if authentication is enabled)
        # client.login("admin", "password")

        # Check daemon health
        health = client.health()
        print(f"✓ Daemon is {health['status']}")

        # Get daemon status
        status = client.status()
        print(f"✓ Version: {status.version}")
        print(f"✓ Uptime: {status.uptime}")
        print(f"✓ Total jobs: {status.total_jobs}")
        print(f"✓ Running jobs: {status.running_jobs}")

        # Define vCenter connection
        vcenter = VCenterConfig(
            server="vcenter.example.com",
            username="administrator@vsphere.local",
            password="your-password",
            insecure=True  # Skip TLS verification for development
        )

        # Create job definition
        job_def = JobDefinition(
            name="Export Ubuntu Server",
            vm_path="/Datacenter/vm/ubuntu-server",
            output_dir="/exports",
            vcenter=vcenter,
            format=ExportFormat.OVF,
            compress=True,
            thin=True
        )

        # Submit job
        print("\nSubmitting job...")
        job_id = client.submit_job(job_def)
        print(f"✓ Job submitted successfully!")
        print(f"  Job ID: {job_id}")

        # Get job details
        job = client.get_job(job_id)
        print(f"  Status: {job.status.value}")

        if job.progress:
            print(f"  Progress: {job.progress.percent_complete}%")
            print(f"  Phase: {job.progress.phase}")

        print(f"\nMonitor job progress with:")
        print(f"  python examples/monitor_jobs.py {job_id}")

    except Exception as e:
        print(f"✗ Error: {e}")
        return 1
    finally:
        client.close()

    return 0


if __name__ == "__main__":
    exit(main())
