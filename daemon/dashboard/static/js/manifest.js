// manifest.js - Handles manifest converter functionality

// Tab switching
document.addEventListener('DOMContentLoaded', function() {
    const tabButtons = document.querySelectorAll('.tab-button');
    const tabContents = document.querySelectorAll('.tab-content');

    console.log('Tab switcher initialized. Found', tabButtons.length, 'tab buttons and', tabContents.length, 'tab contents');

    tabButtons.forEach(button => {
        button.addEventListener('click', () => {
            const targetTab = button.getAttribute('data-tab');
            const targetTabId = targetTab + '-tab';
            const targetElement = document.getElementById(targetTabId);

            console.log('Switching to tab:', targetTab, '(element ID:', targetTabId + ')');

            if (!targetElement) {
                console.error('Tab content element not found:', targetTabId);
                return;
            }

            // Remove active class from all buttons and contents
            tabButtons.forEach(btn => btn.classList.remove('active'));
            tabContents.forEach(content => content.classList.remove('active'));

            // Add active class to clicked button and target content
            button.classList.add('active');
            targetElement.classList.add('active');

            console.log('Tab switched successfully to:', targetTab);
        });
    });

    // Manifest converter form handlers
    const submitManifestJobBtn = document.getElementById('submitManifestJob');
    const generateManifestOnlyBtn = document.getElementById('generateManifestOnly');

    if (submitManifestJobBtn) {
        submitManifestJobBtn.addEventListener('click', submitManifestJob);
    }

    if (generateManifestOnlyBtn) {
        generateManifestOnlyBtn.addEventListener('click', generateManifestOnly);
    }
});

// Submit manifest converter job
async function submitManifestJob() {
    const vmPath = document.getElementById('vmPath').value;
    const outputPath = document.getElementById('outputPath').value;
    const targetFormat = document.getElementById('targetFormat').value;
    const autoConvert = document.getElementById('autoConvert').checked;
    const compress = document.getElementById('compress').checked;
    const verify = document.getElementById('verify').checked;
    const hyper2kvmBinary = document.getElementById('hyper2kvmBinary').value;
    const conversionTimeout = parseInt(document.getElementById('conversionTimeout').value);
    const streamOutput = document.getElementById('streamOutput').checked;

    if (!vmPath || !outputPath) {
        alert('Please fill in VM Path and Output Directory');
        return;
    }

    const jobDefinition = {
        vm_path: vmPath,
        output_path: outputPath,
        format: 'ovf', // Always export as OVF first
        compress: compress,
        options: {
            generate_manifest: true,
            manifest_target_format: targetFormat,
            manifest_checksum: verify,
            auto_convert: autoConvert,
            hyper2kvm_binary: hyper2kvmBinary || undefined,
            conversion_timeout: conversionTimeout * 60, // Convert minutes to seconds
            stream_conversion_output: streamOutput
        }
    };

    try {
        const response = await fetch('/jobs/submit', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(jobDefinition)
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }

        const result = await response.json();
        const jobId = result.job_id;

        // Show status section
        document.getElementById('manifestStatus').style.display = 'block';
        document.getElementById('manifestJobId').textContent = jobId;
        document.getElementById('manifestJobStatus').textContent = 'Pending';
        document.getElementById('manifestProgressText').textContent = '0%';

        // Start monitoring job
        monitorManifestJob(jobId);

        // Scroll to status section
        document.getElementById('manifestStatus').scrollIntoView({ behavior: 'smooth' });

    } catch (error) {
        alert('Failed to submit job: ' + error.message);
    }
}

// Generate manifest only (no conversion)
async function generateManifestOnly() {
    const vmPath = document.getElementById('vmPath').value;
    const outputPath = document.getElementById('outputPath').value;
    const targetFormat = document.getElementById('targetFormat').value;
    const verify = document.getElementById('verify').checked;

    if (!vmPath || !outputPath) {
        alert('Please fill in VM Path and Output Directory');
        return;
    }

    const jobDefinition = {
        vm_path: vmPath,
        output_path: outputPath,
        format: 'ovf',
        options: {
            generate_manifest: true,
            manifest_target_format: targetFormat,
            manifest_checksum: verify,
            auto_convert: false
        }
    };

    try {
        const response = await fetch('/jobs/submit', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(jobDefinition)
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }

        const result = await response.json();
        const jobId = result.job_id;

        // Show status section
        document.getElementById('manifestStatus').style.display = 'block';
        document.getElementById('manifestJobId').textContent = jobId;
        document.getElementById('manifestJobStatus').textContent = 'Pending';
        document.getElementById('manifestProgressText').textContent = '0%';

        // Start monitoring job
        monitorManifestJob(jobId);

        // Scroll to status section
        document.getElementById('manifestStatus').scrollIntoView({ behavior: 'smooth' });

    } catch (error) {
        alert('Failed to submit job: ' + error.message);
    }
}

// Monitor manifest job progress
async function monitorManifestJob(jobId) {
    const interval = setInterval(async () => {
        try {
            const response = await fetch('/jobs/' + jobId);
            if (!response.ok) {
                clearInterval(interval);
                return;
            }

            const job = await response.json();
            updateManifestStatus(job);

            // Stop monitoring if job is complete
            if (job.status === 'completed' || job.status === 'failed' || job.status === 'cancelled') {
                clearInterval(interval);

                if (job.status === 'completed') {
                    showSuccessNotification('Conversion completed successfully!');
                } else if (job.status === 'failed') {
                    showErrorNotification('Conversion failed: ' + (job.error || 'Unknown error'));
                }
            }

        } catch (error) {
            console.error('Failed to fetch job status:', error);
        }
    }, 2000); // Poll every 2 seconds
}

// Update manifest status display
function updateManifestStatus(job) {
    const statusElement = document.getElementById('manifestJobStatus');
    const progressFill = document.getElementById('manifestProgress');
    const progressText = document.getElementById('manifestProgressText');

    // Update status with color
    let statusClass = '';
    switch (job.status) {
        case 'running':
            statusClass = 'status-running';
            break;
        case 'completed':
            statusClass = 'status-completed';
            break;
        case 'failed':
            statusClass = 'status-failed';
            break;
        default:
            statusClass = 'status-pending';
    }

    statusElement.textContent = job.status.toUpperCase();
    statusElement.className = 'status-value ' + statusClass;

    // Update progress
    if (job.progress) {
        const progress = job.progress.percent_complete || 0;
        progressFill.style.width = progress + '%';
        progressText.textContent = progress.toFixed(1) + '%';

        // Add phase info to logs
        if (job.progress.phase) {
            addLogLine('[' + new Date().toLocaleTimeString() + '] Phase: ' + job.progress.phase);
        }
        if (job.progress.current_step) {
            addLogLine('[' + new Date().toLocaleTimeString() + '] ' + job.progress.current_step);
        }
    }

    // Update logs with error if failed
    if (job.status === 'failed' && job.error) {
        addLogLine('[' + new Date().toLocaleTimeString() + '] ERROR: ' + job.error);
    }
}

// Add line to conversion logs
function addLogLine(line) {
    const logsOutput = document.getElementById('logsOutput');
    logsOutput.textContent += line + '\n';
    logsOutput.scrollTop = logsOutput.scrollHeight; // Auto-scroll to bottom
}

// Show success notification
function showSuccessNotification(message) {
    const notification = document.createElement('div');
    notification.className = 'notification notification-success';
    notification.textContent = '✅ ' + message;
    document.body.appendChild(notification);

    setTimeout(() => {
        notification.remove();
    }, 5000);
}

// Show error notification
function showErrorNotification(message) {
    const notification = document.createElement('div');
    notification.className = 'notification notification-error';
    notification.textContent = '❌ ' + message;
    document.body.appendChild(notification);

    setTimeout(() => {
        notification.remove();
    }, 5000);
}
