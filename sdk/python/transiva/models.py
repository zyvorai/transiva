# SPDX-License-Identifier: Apache-2.0

"""HyperSDK data models."""

from dataclasses import dataclass, field, asdict
from datetime import datetime
from enum import Enum
from typing import Optional, List, Dict, Any


class JobStatus(str, Enum):
    """Job status enum."""
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"


class ExportFormat(str, Enum):
    """Export format enum."""
    QCOW2 = "qcow2"
    RAW = "raw"
    VMDK = "vmdk"
    OVA = "ova"
    OVF = "ovf"


class ExportMethod(str, Enum):
    """Export method enum."""
    CTL = "ctl"
    GOVC = "govc"
    OVFTOOL = "ovftool"
    WEB = "web"
    AUTO = ""


@dataclass
class VCenterConfig:
    """vCenter connection configuration."""
    server: str
    username: str
    password: str
    insecure: bool = False

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)


@dataclass
class ExportOptions:
    """Export options configuration."""
    parallel_downloads: int = 4
    remove_cdrom: bool = False
    show_individual_progress: bool = False
    enable_pipeline: bool = False
    h2kvm_path: Optional[str] = None
    hyper2kvm_path: Optional[str] = None  # Deprecated: use h2kvm_path
    pipeline_inspect: bool = False
    pipeline_fix: bool = False
    pipeline_convert: bool = False
    pipeline_validate: bool = False
    pipeline_compress: bool = False
    compress_level: int = 6
    libvirt_integration: bool = False
    libvirt_uri: Optional[str] = None
    libvirt_autostart: bool = False
    libvirt_bridge: Optional[str] = None
    libvirt_pool: Optional[str] = None

    def to_dict(self) -> Dict[str, Any]:
        return {k: v for k, v in asdict(self).items() if v is not None}


@dataclass
class JobDefinition:
    """VM export job definition."""
    vm_path: str
    name: Optional[str] = None
    id: Optional[str] = None
    output_path: Optional[str] = None
    output_dir: Optional[str] = None
    vcenter_url: Optional[str] = None
    vcenter: Optional[VCenterConfig] = None
    username: Optional[str] = None
    datacenter: Optional[str] = None
    format: ExportFormat = ExportFormat.OVF
    export_method: ExportMethod = ExportMethod.AUTO
    method: Optional[str] = None
    compress: bool = False
    thin: bool = False
    insecure: bool = False
    options: Optional[ExportOptions] = None
    created_at: Optional[datetime] = None

    def to_dict(self) -> Dict[str, Any]:
        data = {
            "vm_path": self.vm_path,
        }
        if self.name:
            data["name"] = self.name
        if self.id:
            data["id"] = self.id
        if self.output_path:
            data["output_path"] = self.output_path
        if self.output_dir:
            data["output_dir"] = self.output_dir
        if self.vcenter_url:
            data["vcenter_url"] = self.vcenter_url
        if self.vcenter:
            data["vcenter"] = self.vcenter.to_dict()
        if self.username:
            data["username"] = self.username
        if self.datacenter:
            data["datacenter"] = self.datacenter
        if self.format:
            data["format"] = self.format.value
        if self.export_method:
            data["export_method"] = self.export_method.value
        if self.method:
            data["method"] = self.method
        data["compress"] = self.compress
        data["thin"] = self.thin
        data["insecure"] = self.insecure
        if self.options:
            data["options"] = self.options.to_dict()
        if self.created_at:
            data["created_at"] = self.created_at.isoformat()
        return data


@dataclass
class JobProgress:
    """Job progress information."""
    phase: str
    current_file: Optional[str] = None
    current_step: Optional[str] = None
    files_downloaded: int = 0
    total_files: int = 0
    bytes_downloaded: int = 0
    bytes_transferred: int = 0
    total_bytes: int = 0
    percent_complete: float = 0.0
    estimated_remaining: Optional[str] = None
    export_method: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "JobProgress":
        return cls(**{k: v for k, v in data.items() if k in cls.__annotations__})


@dataclass
class JobResult:
    """Job result information."""
    vm_name: str
    output_dir: str
    ovf_path: str
    files: List[str]
    total_size: int
    duration: int
    success: bool
    export_method: Optional[str] = None
    error: Optional[str] = None
    output_files: Optional[List[str]] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "JobResult":
        return cls(
            vm_name=data["vm_name"],
            output_dir=data["output_dir"],
            ovf_path=data["ovf_path"],
            files=data.get("files", []),
            total_size=data["total_size"],
            duration=data["duration"],
            success=data["success"],
            export_method=data.get("export_method"),
            error=data.get("error"),
            output_files=data.get("output_files"),
        )


@dataclass
class Job:
    """Complete job information."""
    definition: JobDefinition
    status: JobStatus
    updated_at: datetime
    progress: Optional[JobProgress] = None
    result: Optional[JobResult] = None
    error: Optional[str] = None
    started_at: Optional[datetime] = None
    completed_at: Optional[datetime] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Job":
        definition_data = data["definition"]
        definition = JobDefinition(
            vm_path=definition_data["vm_path"],
            name=definition_data.get("name"),
            id=definition_data.get("id"),
            output_path=definition_data.get("output_path"),
            output_dir=definition_data.get("output_dir"),
            vcenter_url=definition_data.get("vcenter_url"),
            username=definition_data.get("username"),
            datacenter=definition_data.get("datacenter"),
            format=ExportFormat(definition_data.get("format", "ovf")),
            export_method=ExportMethod(definition_data.get("export_method", "")),
            compress=definition_data.get("compress", False),
            thin=definition_data.get("thin", False),
            insecure=definition_data.get("insecure", False),
        )

        progress = None
        if data.get("progress"):
            progress = JobProgress.from_dict(data["progress"])

        result = None
        if data.get("result"):
            result = JobResult.from_dict(data["result"])

        return cls(
            definition=definition,
            status=JobStatus(data["status"]),
            updated_at=datetime.fromisoformat(data["updated_at"].replace("Z", "+00:00")),
            progress=progress,
            result=result,
            error=data.get("error"),
            started_at=datetime.fromisoformat(data["started_at"].replace("Z", "+00:00")) if data.get("started_at") else None,
            completed_at=datetime.fromisoformat(data["completed_at"].replace("Z", "+00:00")) if data.get("completed_at") else None,
        )


@dataclass
class ScheduledJob:
    """Scheduled job configuration."""
    name: str
    schedule: str
    job_template: JobDefinition
    id: Optional[str] = None
    description: Optional[str] = None
    enabled: bool = True
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None
    next_run: Optional[datetime] = None
    last_run: Optional[datetime] = None
    run_count: int = 0
    tags: List[str] = field(default_factory=list)

    def to_dict(self) -> Dict[str, Any]:
        data = {
            "name": self.name,
            "schedule": self.schedule,
            "job_template": self.job_template.to_dict(),
            "enabled": self.enabled,
            "run_count": self.run_count,
        }
        if self.id:
            data["id"] = self.id
        if self.description:
            data["description"] = self.description
        if self.tags:
            data["tags"] = self.tags
        return data

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ScheduledJob":
        job_template_data = data["job_template"]
        job_template = JobDefinition(
            vm_path=job_template_data["vm_path"],
            name=job_template_data.get("name"),
        )

        return cls(
            name=data["name"],
            schedule=data["schedule"],
            job_template=job_template,
            id=data.get("id"),
            description=data.get("description"),
            enabled=data.get("enabled", True),
            run_count=data.get("run_count", 0),
            tags=data.get("tags", []),
        )


@dataclass
class Webhook:
    """Webhook configuration."""
    url: str
    events: List[str] = field(default_factory=lambda: ["job_completed", "job_failed"])
    headers: Dict[str, str] = field(default_factory=dict)

    def to_dict(self) -> Dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Webhook":
        return cls(**data)


@dataclass
class DaemonStatus:
    """Daemon status information."""
    version: str
    uptime: str
    total_jobs: int
    running_jobs: int
    completed_jobs: int
    failed_jobs: int
    cancelled_jobs: int
    timestamp: datetime

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "DaemonStatus":
        return cls(
            version=data["version"],
            uptime=data["uptime"],
            total_jobs=data["total_jobs"],
            running_jobs=data["running_jobs"],
            completed_jobs=data["completed_jobs"],
            failed_jobs=data["failed_jobs"],
            cancelled_jobs=data["cancelled_jobs"],
            timestamp=datetime.fromisoformat(data["timestamp"].replace("Z", "+00:00")),
        )


# Carbon-Aware Models


@dataclass
class CarbonForecast:
    """Carbon intensity forecast."""
    time: datetime
    intensity_gco2_kwh: float
    quality: str

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CarbonForecast":
        return cls(
            time=datetime.fromisoformat(data["time"].replace("Z", "+00:00")),
            intensity_gco2_kwh=data["intensity_gco2_kwh"],
            quality=data["quality"],
        )


@dataclass
class CarbonStatus:
    """Grid carbon status."""
    zone: str
    current_intensity: float
    renewable_percent: float
    optimal_for_backup: bool
    next_optimal_time: Optional[datetime]
    forecast: List[CarbonForecast]
    reasoning: str
    quality: str
    timestamp: datetime

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CarbonStatus":
        forecast = [CarbonForecast.from_dict(f) for f in data.get("forecast_next_4h", [])]
        next_optimal = None
        if data.get("next_optimal_time"):
            next_optimal = datetime.fromisoformat(data["next_optimal_time"].replace("Z", "+00:00"))

        return cls(
            zone=data["zone"],
            current_intensity=data["current_intensity"],
            renewable_percent=data["renewable_percent"],
            optimal_for_backup=data["optimal_for_backup"],
            next_optimal_time=next_optimal,
            forecast=forecast,
            reasoning=data["reasoning"],
            quality=data["quality"],
            timestamp=datetime.fromisoformat(data["timestamp"].replace("Z", "+00:00")),
        )


@dataclass
class CarbonReport:
    """Carbon footprint report."""
    operation_id: str
    start_time: datetime
    end_time: datetime
    duration_hours: float
    data_size_gb: float
    energy_kwh: float
    carbon_intensity_gco2_kwh: float
    carbon_emissions_kg_co2: float
    savings_vs_worst_kg_co2: float
    renewable_percent: float
    equivalent: str

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CarbonReport":
        return cls(
            operation_id=data["operation_id"],
            start_time=datetime.fromisoformat(data["start_time"].replace("Z", "+00:00")),
            end_time=datetime.fromisoformat(data["end_time"].replace("Z", "+00:00")),
            duration_hours=data["duration_hours"],
            data_size_gb=data["data_size_gb"],
            energy_kwh=data["energy_kwh"],
            carbon_intensity_gco2_kwh=data["carbon_intensity_gco2_kwh"],
            carbon_emissions_kg_co2=data["carbon_emissions_kg_co2"],
            savings_vs_worst_kg_co2=data["savings_vs_worst_kg_co2"],
            renewable_percent=data["renewable_percent"],
            equivalent=data["equivalent"],
        )


@dataclass
class CarbonZone:
    """Carbon zone information."""
    id: str
    name: str
    region: str
    description: str
    typical_intensity: float

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CarbonZone":
        return cls(
            id=data["id"],
            name=data["name"],
            region=data["region"],
            description=data["description"],
            typical_intensity=data["typical_intensity"],
        )


@dataclass
class CarbonEstimate:
    """Carbon savings estimate."""
    current_intensity_gco2_kwh: float
    current_emissions_kg_co2: float
    best_intensity_gco2_kwh: float
    best_emissions_kg_co2: float
    best_time: Optional[datetime]
    savings_kg_co2: float
    savings_percent: float
    recommendation: str
    delay_minutes: Optional[float]
    forecast: List[CarbonForecast]

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CarbonEstimate":
        forecast = [CarbonForecast.from_dict(f) for f in data.get("forecast", [])]
        best_time = None
        if data.get("best_time"):
            best_time = datetime.fromisoformat(data["best_time"].replace("Z", "+00:00"))

        return cls(
            current_intensity_gco2_kwh=data["current_intensity_gco2_kwh"],
            current_emissions_kg_co2=data["current_emissions_kg_co2"],
            best_intensity_gco2_kwh=data["best_intensity_gco2_kwh"],
            best_emissions_kg_co2=data["best_emissions_kg_co2"],
            best_time=best_time,
            savings_kg_co2=data["savings_kg_co2"],
            savings_percent=data["savings_percent"],
            recommendation=data["recommendation"],
            delay_minutes=data.get("delay_minutes"),
            forecast=forecast,
        )
