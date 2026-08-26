#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0

"""Example: Monitor job progress in real-time."""

import sys
import time
from transiva import HyperSDK, JobStatus


def format_bytes(bytes_val):
    """Format bytes to human-readable format."""
    for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
        if bytes_val < 1024.0:
            return f"{bytes_val:.2f} {unit}"
        bytes_val /= 1024.0
    return f"{bytes_val:.2f} PB"


def monitor_job(client, job_id):
    """Monitor a single job until completion."""
    print(f"Monitoring job: {job_id}\n")

    while True:
        try:
            job = client.get_job(job_id)

            # Clear line and print status
            print(f"\r\033[K", end="")  # Clear current line
            print(f"Status: {job.status.value}", end="")

            if job.progress:
                progress = job.progress
                print(f" | Progress: {progress.percent_complete:.1f}%", end="")
                print(f" | Phase: {progress.phase}", end="")

                if progress.current_file:
                    print(f" | File: {progress.current_file}", end="")

                if progress.bytes_downloaded and progress.total_bytes:
                    downloaded = format_bytes(progress.bytes_downloaded)
                    total = format_bytes(progress.total_bytes)
                    print(f" | {downloaded}/{total}", end="")

                if progress.estimated_remaining:
                    print(f" | ETA: {progress.estimated_remaining}", end="")

            # Check if job is complete
            if job.status in [JobStatus.COMPLETED, JobStatus.FAILED, JobStatus.CANCELLED]:
                print()  # New line
                break

            time.sleep(2)  # Poll every 2 seconds

        except KeyboardInterrupt:
            print("\n\nCancelling job...")
            client.cancel_job(job_id)
            print("Job cancelled.")
            return 1
        except Exception as e:
            print(f"\nError: {e}")
            return 1

    # Print final result
    print(f"\n{'='*80}")
    if job.status == JobStatus.COMPLETED:
        print("✓ Job completed successfully!")
        if job.result:
            result = job.result
            print(f"  VM Name: {result.vm_name}")
            print(f"  Output Directory: {result.output_dir}")
            print(f"  Total Size: {format_bytes(result.total_size)}")
            print(f"  Duration: {result.duration / 1e9:.2f} seconds")
            print(f"  Export Method: {result.export_method}")
            print(f"  Files:")
            for file in result.files:
                print(f"    - {file}")
    elif job.status == JobStatus.FAILED:
        print("✗ Job failed!")
        if job.error:
            print(f"  Error: {job.error}")
    elif job.status == JobStatus.CANCELLED:
        print("⊘ Job was cancelled")

    return 0


def monitor_all_jobs(client):
    """Monitor all running jobs."""
    print("Monitoring all jobs...\n")

    while True:
        try:
            # Get all jobs
            jobs = client.list_jobs(all=True)

            # Filter running jobs
            running_jobs = [j for j in jobs if j.status == JobStatus.RUNNING]

            if not running_jobs:
                print("No running jobs.")
                break

            # Clear screen
            print("\033[2J\033[H", end="")

            # Print header
            print(f"{'='*80}")
            print(f"Running Jobs: {len(running_jobs)}")
            print(f"{'='*80}\n")

            # Print each job
            for job in running_jobs:
                job_id = job.definition.id or "Unknown"
                name = job.definition.name or job.definition.vm_path
                print(f"Job: {job_id} - {name}")

                if job.progress:
                    progress = job.progress
                    bar_length = 40
                    filled = int(bar_length * progress.percent_complete / 100)
                    bar = '█' * filled + '░' * (bar_length - filled)
                    print(f"  [{bar}] {progress.percent_complete:.1f}%")
                    print(f"  Phase: {progress.phase}")

                    if progress.estimated_remaining:
                        print(f"  ETA: {progress.estimated_remaining}")

                print()

            time.sleep(3)  # Refresh every 3 seconds

        except KeyboardInterrupt:
            print("\nStopped monitoring.")
            return 0
        except Exception as e:
            print(f"\nError: {e}")
            return 1


def main():
    if len(sys.argv) > 1:
        job_id = sys.argv[1]
        mode = "single"
    else:
        job_id = None
        mode = "all"

    # Initialize client
    client = HyperSDK("http://localhost:8080")

    try:
        # Login (if authentication is enabled)
        # client.login("admin", "password")

        if mode == "single":
            return monitor_job(client, job_id)
        else:
            return monitor_all_jobs(client)

    finally:
        client.close()


if __name__ == "__main__":
    exit(main())
