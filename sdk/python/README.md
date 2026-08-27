# HyperSDK Python Client

Python client library for the HyperSDK VM migration and export platform.

## Features

- 🚀 Simple, intuitive API
- 🔐 Built-in authentication support
- 📝 Full type hints for IDE autocomplete
- ✅ Comprehensive error handling
- 📦 Support for all HyperSDK operations:
  - Job submission and monitoring
  - VM discovery and operations
  - Schedule management
  - Webhook configuration
  - Libvirt integration
  - H2KVM conversion
  - **Carbon-aware scheduling** (NEW in v2.0)

## Installation

```bash
pip install transiva
```

Or install from source:

```bash
cd sdk/python
pip install -e .
```

## Quick Start

### Basic Usage

```python
from transiva import HyperSDK, JobDefinition, VCenterConfig

# Initialize client
client = HyperSDK("http://localhost:8080")

# Login (if authentication is enabled)
client.login("admin", "password")

# Check daemon status
status = client.status()
print(f"Daemon version: {status.version}")
print(f"Running jobs: {status.running_jobs}")

# Submit a VM export job
job_def = JobDefinition(
    vm_path="/Datacenter/vm/my-virtual-machine",
    output_dir="/exports",
    vcenter=VCenterConfig(
        server="vcenter.example.com",
        username="administrator@vsphere.local",
        password="your-password",
        insecure=True  # Skip TLS verification
    ),
    format="ovf",  # or "ova", "qcow2", "vmdk"
    compress=True
)

job_id = client.submit_job(job_def)
print(f"Job submitted: {job_id}")

# Monitor job progress
job = client.get_job(job_id)
print(f"Job status: {job.status}")

if job.progress:
    print(f"Progress: {job.progress.percent_complete}%")
    print(f"Phase: {job.progress.phase}")
```

### List All Jobs

```python
# Get all jobs
jobs = client.list_jobs(all=True)

for job in jobs:
    print(f"Job {job.definition.id}: {job.status}")
    if job.progress:
        print(f"  Progress: {job.progress.percent_complete}%")
```

### Filter Jobs by Status

```python
from transiva import JobStatus

# Get only running jobs
running_jobs = client.query_jobs(status=[JobStatus.RUNNING])

for job in running_jobs:
    progress = client.get_job_progress(job.definition.id)
    print(f"{job.definition.name}: {progress.percent_complete}% complete")
    print(f"  ETA: {progress.estimated_remaining}")
```

### Cancel a Job

```python
success = client.cancel_job(job_id)
if success:
    print("Job cancelled successfully")
```

### Scheduled Jobs

```python
from transiva import ScheduledJob, JobDefinition

# Create a scheduled job (runs daily at 2 AM)
schedule = ScheduledJob(
    name="Daily VM Backup",
    description="Backup production VMs every night",
    schedule="0 2 * * *",  # Cron format
    job_template=JobDefinition(
        vm_path="/Datacenter/vm/production-vm",
        output_dir="/backups",
        format="ova",
        compress=True
    ),
    enabled=True,
    tags=["backup", "production"]
)

created_schedule = client.create_schedule(schedule)
print(f"Schedule created: {created_schedule.id}")
print(f"Next run: {created_schedule.next_run}")

# List all schedules
schedules = client.list_schedules()
for sched in schedules:
    print(f"{sched.name}: {sched.schedule} (enabled={sched.enabled})")

# Manually trigger a schedule
client.trigger_schedule(created_schedule.id)
```

### Webhooks

```python
from transiva import Webhook

# Add a webhook for job completion notifications
webhook = Webhook(
    url="https://myapp.example.com/webhook",
    events=["job_completed", "job_failed"],
    headers={
        "Authorization": "Bearer my-webhook-token",
        "X-Custom-Header": "value"
    }
)

client.add_webhook(webhook)

# Test the webhook
client.test_webhook("https://myapp.example.com/webhook")
```

### VM Operations

```python
from transiva import VCenterConfig

vcenter = VCenterConfig(
    server="vcenter.example.com",
    username="admin",
    password="password",
    insecure=True
)

# List VMs
vms = client.list_vms(vcenter.to_dict())
for vm in vms:
    print(f"VM: {vm['name']} - {vm['power_state']}")

# Get VM details
vm_info = client.get_vm_info(vcenter.to_dict(), "/Datacenter/vm/my-vm")
print(f"CPU: {vm_info['cpu']}, Memory: {vm_info['memory_mb']} MB")

# Shutdown a VM
client.shutdown_vm(vcenter.to_dict(), "/Datacenter/vm/my-vm")
```

### Libvirt Integration

```python
# List libvirt domains
domains = client.list_domains()
for domain in domains:
    print(f"Domain: {domain['name']} - {domain['state']}")

# Start a domain
client.start_domain("my-vm")

# Create a snapshot
client.create_snapshot(
    domain="my-vm",
    name="before-update",
    description="Snapshot before system update"
)

# List snapshots
snapshots = client.list_snapshots("my-vm")
for snapshot in snapshots:
    print(f"Snapshot: {snapshot['name']}")
```

### H2KVM Conversion

```python
# Convert a VM
conversion_id = client.convert_vm(
    source_path="/exports/vm.ovf",
    output_path="/converted/vm.qcow2"
)

# Check conversion status
status = client.get_conversion_status(conversion_id)
print(f"Conversion status: {status}")
```

### Carbon-Aware Scheduling (NEW in v2.0) 🌿

Reduce carbon emissions from VM backups by 30-50% through intelligent scheduling based on grid carbon intensity.

#### Check Grid Carbon Status

```python
# Check current grid status
status = client.get_carbon_status(zone="US-CAL-CISO", threshold=200)

print(f"Carbon Intensity: {status.current_intensity:.0f} gCO2/kWh")
print(f"Quality: {status.quality}")  # excellent, good, moderate, poor, very poor
print(f"Optimal for Backup: {status.optimal_for_backup}")
print(f"Renewable Energy: {status.renewable_percent:.1f}%")
print(f"Reasoning: {status.reasoning}")

# View 4-hour forecast
for forecast in status.forecast:
    print(f"{forecast.time.strftime('%H:%M')}: {forecast.intensity_gco2_kwh:.0f} gCO2/kWh ({forecast.quality})")

# Next optimal time
if status.next_optimal_time:
    print(f"Next clean period: {status.next_optimal_time.strftime('%H:%M')}")
```

#### List Available Carbon Zones

```python
# List all zones (12 global zones: US, EU, APAC)
zones = client.list_carbon_zones()

for zone in zones:
    print(f"{zone.id}: {zone.name} ({zone.region})")
    print(f"  Typical Intensity: {zone.typical_intensity:.0f} gCO2/kWh")
```

#### Estimate Carbon Savings

```python
# Estimate savings from delaying backup
estimate = client.estimate_carbon_savings(
    zone="US-CAL-CISO",
    data_size_gb=500.0,
    duration_hours=2.0
)

print(f"Run Now: {estimate.current_emissions_kg_co2:.3f} kg CO2")
print(f"Run Later: {estimate.best_emissions_kg_co2:.3f} kg CO2")
print(f"Savings: {estimate.savings_kg_co2:.3f} kg CO2 ({estimate.savings_percent:.1f}%)")
print(f"Delay: {estimate.delay_minutes:.0f} minutes")
print(f"Recommendation: {estimate.recommendation}")
```

#### Submit Carbon-Aware Job

```python
from transiva import JobDefinition

job_def = JobDefinition(
    vm_path="/datacenter/vm/prod-db",
    output_dir="/backups"
)

# Submit with carbon-awareness
# Job will be delayed if grid is dirty
job_id = client.submit_carbon_aware_job(
    job_def,
    carbon_zone="US-CAL-CISO",
    max_intensity=200.0,  # gCO2/kWh threshold
    max_delay_hours=4.0   # Maximum delay allowed
)

print(f"Job ID: {job_id}")
# If grid is dirty, job will automatically be delayed for cleaner period
```

#### Generate Carbon Report

```python
from datetime import datetime, timedelta

# Get carbon footprint report for completed job
report = client.get_carbon_report(
    job_id="job-123",
    start_time=datetime.now() - timedelta(hours=2),
    end_time=datetime.now(),
    data_size_gb=500.0,
    zone="US-CAL-CISO"
)

print(f"Energy Used: {report.energy_kwh:.3f} kWh")
print(f"Carbon Emissions: {report.carbon_emissions_kg_co2:.3f} kg CO2")
print(f"Renewable Energy: {report.renewable_percent:.1f}%")
print(f"Savings vs Worst: {report.savings_vs_worst_kg_co2:.3f} kg CO2")
print(f"Equivalent: {report.equivalent}")
# Example: "0.1 km of driving"
```

#### Complete Workflow Example

```python
from transiva import HyperSDK, JobDefinition

client = HyperSDK("http://localhost:8080")

# 1. Check grid status
status = client.get_carbon_status(zone="US-CAL-CISO")

# 2. Estimate savings
estimate = client.estimate_carbon_savings(
    zone="US-CAL-CISO",
    data_size_gb=500,
    duration_hours=2
)

# 3. Make decision
job_def = JobDefinition(
    vm_path="/datacenter/vm/prod",
    output_dir="/backups"
)

if status.optimal_for_backup:
    print("✅ Grid is clean - running backup now")
    job_id = client.submit_job(job_def)
elif estimate.savings_percent > 30:
    print(f"⏰ Grid is dirty - delaying for {estimate.delay_minutes:.0f} min")
    print(f"   Expected savings: {estimate.savings_percent:.1f}%")
    job_id = client.submit_carbon_aware_job(job_def, max_delay_hours=4)
else:
    print("⚠️  Running now despite dirty grid (savings < 30%)")
    job_id = client.submit_job(job_def)

print(f"Job ID: {job_id}")
```

**See `examples/carbon_aware_backup.py` for a complete example with all features!**

### Context Manager

```python
# Automatically close the client when done
with HyperSDK("http://localhost:8080") as client:
    client.login("admin", "password")
    jobs = client.list_jobs()
    print(f"Total jobs: {len(jobs)}")
```

## Advanced Usage

### Custom Timeout and SSL Verification

```python
client = HyperSDK(
    base_url="https://transiva.example.com",
    api_key="your-api-key",
    timeout=60,  # 60 second timeout
    verify_ssl=False  # Skip SSL verification (not recommended for production)
)
```

### Export with Advanced Options

```python
from transiva import JobDefinition, ExportOptions

job_def = JobDefinition(
    vm_path="/Datacenter/vm/my-vm",
    output_dir="/exports",
    options=ExportOptions(
        parallel_downloads=8,  # Download 8 files in parallel
        remove_cdrom=True,  # Remove CD-ROM devices
        show_individual_progress=True,
        enable_pipeline=True,  # Enable h2kvm pipeline
        pipeline_convert=True,
        pipeline_validate=True,
        libvirt_integration=True,
        libvirt_uri="qemu:///system",
        libvirt_pool="default"
    )
)

job_id = client.submit_job(job_def)
```

### Batch Job Submission

```python
# Submit multiple jobs at once
jobs = [
    JobDefinition(vm_path="/Datacenter/vm/vm1", output_dir="/exports"),
    JobDefinition(vm_path="/Datacenter/vm/vm2", output_dir="/exports"),
    JobDefinition(vm_path="/Datacenter/vm/vm3", output_dir="/exports"),
]

job_ids = client.submit_jobs(jobs)
print(f"Submitted {len(job_ids)} jobs")
```

### Error Handling

```python
from transiva import HyperSDKError, AuthenticationError, JobNotFoundError, APIError

try:
    client = HyperSDK("http://localhost:8080")
    client.login("admin", "wrong-password")
except AuthenticationError as e:
    print(f"Login failed: {e}")

try:
    job = client.get_job("non-existent-job")
except JobNotFoundError as e:
    print(f"Job not found: {e}")

try:
    job_def = JobDefinition(vm_path="/invalid/path")
    job_id = client.submit_job(job_def)
except APIError as e:
    print(f"API error: {e}")
    print(f"Status code: {e.status_code}")
    print(f"Response: {e.response}")
```

## API Reference

### Client Methods

#### Authentication
- `login(username, password)` - Login and obtain session token
- `logout()` - Logout and invalidate session

#### Health & Status
- `health()` - Check API health
- `status()` - Get daemon status
- `capabilities()` - Get export capabilities

#### Job Management
- `submit_job(job_def)` - Submit a single job
- `submit_jobs(job_defs)` - Submit multiple jobs
- `get_job(job_id)` - Get job details
- `list_jobs(all=True)` - List all jobs
- `query_jobs(job_ids, status, all, limit)` - Query jobs with filters
- `cancel_job(job_id)` - Cancel a job
- `cancel_jobs(job_ids)` - Cancel multiple jobs
- `get_job_progress(job_id)` - Get job progress
- `get_job_logs(job_id)` - Get job logs
- `get_job_eta(job_id)` - Get job ETA

#### VM Operations
- `list_vms(vcenter_config)` - List VMs
- `get_vm_info(vcenter_config, vm_path)` - Get VM info
- `shutdown_vm(vcenter_config, vm_path)` - Shutdown VM

#### Schedule Management
- `list_schedules()` - List schedules
- `create_schedule(schedule)` - Create schedule
- `get_schedule(schedule_id)` - Get schedule
- `update_schedule(schedule_id, schedule)` - Update schedule
- `delete_schedule(schedule_id)` - Delete schedule
- `enable_schedule(schedule_id)` - Enable schedule
- `disable_schedule(schedule_id)` - Disable schedule
- `trigger_schedule(schedule_id)` - Trigger schedule

#### Webhook Management
- `list_webhooks()` - List webhooks
- `add_webhook(webhook)` - Add webhook
- `test_webhook(url)` - Test webhook
- `delete_webhook(webhook_id)` - Delete webhook

#### Libvirt Operations
- `list_domains()` - List domains
- `get_domain(name)` - Get domain
- `start_domain(name)` - Start domain
- `shutdown_domain(name)` - Shutdown domain
- `list_snapshots(domain)` - List snapshots
- `create_snapshot(domain, name, description)` - Create snapshot

#### H2KVM Integration
- `convert_vm(source_path, output_path)` - Convert VM
- `get_conversion_status(conversion_id)` - Get conversion status

#### Carbon-Aware Scheduling (NEW in v2.0)
- `get_carbon_status(zone, threshold)` - Get grid carbon status
- `list_carbon_zones()` - List available carbon zones
- `estimate_carbon_savings(zone, data_size_gb, duration_hours)` - Estimate carbon savings
- `get_carbon_report(job_id, start_time, end_time, data_size_gb, zone)` - Generate carbon report
- `submit_carbon_aware_job(job_def, carbon_zone, max_intensity, max_delay_hours)` - Submit carbon-aware job

## Development

### Running Tests

```bash
pip install -e ".[dev]"
pytest
```

### Code Formatting

```bash
black transiva/
```

### Type Checking

```bash
mypy transiva/
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

Apache-2.0

## Support

- **Issues**: https://github.com/zyvorai/transiva/issues
- **Documentation**: https://github.com/zyvorai/transiva
