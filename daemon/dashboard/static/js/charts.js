// HyperSDK Dashboard Charts - Lightweight Canvas-based Charting

/**
 * Simple line chart implementation using HTML5 Canvas
 * No external dependencies required
 */
class LineChart {
    constructor(canvasId, options = {}) {
        this.canvas = document.getElementById(canvasId);
        if (!this.canvas) {
            console.error(`Canvas element ${canvasId} not found`);
            return;
        }

        this.ctx = this.canvas.getContext('2d');
        this.options = {
            title: options.title || '',
            xLabel: options.xLabel || '',
            yLabel: options.yLabel || '',
            lineColor: options.lineColor || '#f97316',
            fillColor: options.fillColor || 'rgba(249, 115, 22, 0.1)',
            gridColor: options.gridColor || '#334155',
            textColor: options.textColor || '#94a3b8',
            backgroundColor: options.backgroundColor || '#1e293b',
            padding: options.padding || 50,
            showGrid: options.showGrid !== false,
            showPoints: options.showPoints !== false,
            smooth: options.smooth !== false,
            ...options
        };

        this.data = [];
        this.resize();
        window.addEventListener('resize', () => this.resize());
    }

    resize() {
        const rect = this.canvas.getBoundingClientRect();
        this.canvas.width = rect.width * window.devicePixelRatio;
        this.canvas.height = rect.height * window.devicePixelRatio;
        this.ctx.scale(window.devicePixelRatio, window.devicePixelRatio);
        this.width = rect.width;
        this.height = rect.height;

        if (this.data.length > 0) {
            this.draw();
        }
    }

    setData(data) {
        this.data = data;
        this.draw();
    }

    draw() {
        if (!this.data || this.data.length === 0) {
            this.drawEmptyState();
            return;
        }

        const { padding } = this.options;
        const chartWidth = this.width - (padding * 2);
        const chartHeight = this.height - (padding * 2);

        // Clear canvas
        this.ctx.fillStyle = this.options.backgroundColor;
        this.ctx.fillRect(0, 0, this.width, this.height);

        // Calculate scales
        const maxValue = Math.max(...this.data.map(d => d.value));
        const minValue = Math.min(...this.data.map(d => d.value), 0);
        const valueRange = maxValue - minValue || 1;

        const xScale = chartWidth / (this.data.length - 1 || 1);
        const yScale = chartHeight / valueRange;

        // Draw grid
        if (this.options.showGrid) {
            this.drawGrid(padding, chartWidth, chartHeight, maxValue, minValue, valueRange);
        }

        // Draw axes
        this.drawAxes(padding, chartWidth, chartHeight);

        // Draw line with fill
        this.drawLine(padding, chartHeight, xScale, yScale, minValue);

        // Draw points
        if (this.options.showPoints) {
            this.drawPoints(padding, chartHeight, xScale, yScale, minValue);
        }

        // Draw labels
        this.drawLabels(padding, chartWidth, chartHeight);
    }

    drawGrid(padding, chartWidth, chartHeight, maxValue, minValue, valueRange) {
        this.ctx.strokeStyle = this.options.gridColor;
        this.ctx.lineWidth = 1;
        this.ctx.setLineDash([5, 5]);

        // Horizontal grid lines
        const gridLines = 5;
        for (let i = 0; i <= gridLines; i++) {
            const y = padding + (chartHeight / gridLines) * i;
            this.ctx.beginPath();
            this.ctx.moveTo(padding, y);
            this.ctx.lineTo(padding + chartWidth, y);
            this.ctx.stroke();

            // Y-axis labels
            const value = maxValue - (valueRange / gridLines) * i;
            this.ctx.fillStyle = this.options.textColor;
            this.ctx.font = '12px sans-serif';
            this.ctx.textAlign = 'right';
            this.ctx.fillText(Math.round(value), padding - 10, y + 4);
        }

        this.ctx.setLineDash([]);
    }

    drawAxes(padding, chartWidth, chartHeight) {
        this.ctx.strokeStyle = this.options.gridColor;
        this.ctx.lineWidth = 2;

        // Y-axis
        this.ctx.beginPath();
        this.ctx.moveTo(padding, padding);
        this.ctx.lineTo(padding, padding + chartHeight);
        this.ctx.stroke();

        // X-axis
        this.ctx.beginPath();
        this.ctx.moveTo(padding, padding + chartHeight);
        this.ctx.lineTo(padding + chartWidth, padding + chartHeight);
        this.ctx.stroke();
    }

    drawLine(padding, chartHeight, xScale, yScale, minValue) {
        // Draw filled area
        this.ctx.fillStyle = this.options.fillColor;
        this.ctx.beginPath();
        this.ctx.moveTo(padding, padding + chartHeight);

        this.data.forEach((point, i) => {
            const x = padding + (i * xScale);
            const y = padding + chartHeight - ((point.value - minValue) * yScale);

            if (i === 0) {
                this.ctx.lineTo(x, y);
            } else if (this.options.smooth) {
                const prevPoint = this.data[i - 1];
                const prevX = padding + ((i - 1) * xScale);
                const prevY = padding + chartHeight - ((prevPoint.value - minValue) * yScale);

                const cpX = (prevX + x) / 2;
                this.ctx.quadraticCurveTo(prevX, prevY, cpX, (prevY + y) / 2);
                this.ctx.quadraticCurveTo(cpX, (prevY + y) / 2, x, y);
            } else {
                this.ctx.lineTo(x, y);
            }
        });

        this.ctx.lineTo(padding + chartWidth, padding + chartHeight);
        this.ctx.closePath();
        this.ctx.fill();

        // Draw line
        this.ctx.strokeStyle = this.options.lineColor;
        this.ctx.lineWidth = 2;
        this.ctx.beginPath();

        this.data.forEach((point, i) => {
            const x = padding + (i * xScale);
            const y = padding + chartHeight - ((point.value - minValue) * yScale);

            if (i === 0) {
                this.ctx.moveTo(x, y);
            } else if (this.options.smooth) {
                const prevPoint = this.data[i - 1];
                const prevX = padding + ((i - 1) * xScale);
                const prevY = padding + chartHeight - ((prevPoint.value - minValue) * yScale);

                const cpX = (prevX + x) / 2;
                this.ctx.quadraticCurveTo(prevX, prevY, cpX, (prevY + y) / 2);
                this.ctx.quadraticCurveTo(cpX, (prevY + y) / 2, x, y);
            } else {
                this.ctx.lineTo(x, y);
            }
        });

        this.ctx.stroke();
    }

    drawPoints(padding, chartHeight, xScale, yScale, minValue) {
        this.data.forEach((point, i) => {
            const x = padding + (i * xScale);
            const y = padding + chartHeight - ((point.value - minValue) * yScale);

            // Point circle
            this.ctx.fillStyle = this.options.lineColor;
            this.ctx.beginPath();
            this.ctx.arc(x, y, 4, 0, Math.PI * 2);
            this.ctx.fill();

            // Point outline
            this.ctx.strokeStyle = this.options.backgroundColor;
            this.ctx.lineWidth = 2;
            this.ctx.stroke();
        });
    }

    drawLabels(padding, chartWidth, chartHeight) {
        this.ctx.fillStyle = this.options.textColor;
        this.ctx.font = '12px sans-serif';

        // X-axis labels (show every nth label to avoid crowding)
        const labelInterval = Math.ceil(this.data.length / 6);
        this.data.forEach((point, i) => {
            if (i % labelInterval === 0 || i === this.data.length - 1) {
                const x = padding + (i * (chartWidth / (this.data.length - 1 || 1)));
                this.ctx.textAlign = 'center';
                this.ctx.fillText(point.label, x, this.height - padding + 20);
            }
        });

        // Title
        if (this.options.title) {
            this.ctx.font = 'bold 14px sans-serif';
            this.ctx.textAlign = 'center';
            this.ctx.fillText(this.options.title, this.width / 2, 20);
        }

        // Y-axis label
        if (this.options.yLabel) {
            this.ctx.save();
            this.ctx.translate(15, this.height / 2);
            this.ctx.rotate(-Math.PI / 2);
            this.ctx.textAlign = 'center';
            this.ctx.font = '12px sans-serif';
            this.ctx.fillText(this.options.yLabel, 0, 0);
            this.ctx.restore();
        }
    }

    drawEmptyState() {
        this.ctx.fillStyle = this.options.backgroundColor;
        this.ctx.fillRect(0, 0, this.width, this.height);

        this.ctx.fillStyle = this.options.textColor;
        this.ctx.font = '14px sans-serif';
        this.ctx.textAlign = 'center';
        this.ctx.fillText('No data available', this.width / 2, this.height / 2);
    }
}

/**
 * Doughnut chart for showing distribution
 */
class DoughnutChart {
    constructor(canvasId, options = {}) {
        this.canvas = document.getElementById(canvasId);
        if (!this.canvas) {
            console.error(`Canvas element ${canvasId} not found`);
            return;
        }

        this.ctx = this.canvas.getContext('2d');
        this.options = {
            title: options.title || '',
            colors: options.colors || ['#f97316', '#3b82f6', '#10b981', '#f59e0b', '#ef4444'],
            backgroundColor: options.backgroundColor || '#1e293b',
            textColor: options.textColor || '#94a3b8',
            showLegend: options.showLegend !== false,
            ...options
        };

        this.data = [];
        this.resize();
        window.addEventListener('resize', () => this.resize());
    }

    resize() {
        const rect = this.canvas.getBoundingClientRect();
        this.canvas.width = rect.width * window.devicePixelRatio;
        this.canvas.height = rect.height * window.devicePixelRatio;
        this.ctx.scale(window.devicePixelRatio, window.devicePixelRatio);
        this.width = rect.width;
        this.height = rect.height;

        if (this.data.length > 0) {
            this.draw();
        }
    }

    setData(data) {
        this.data = data;
        this.draw();
    }

    draw() {
        if (!this.data || this.data.length === 0) {
            this.drawEmptyState();
            return;
        }

        // Clear canvas
        this.ctx.fillStyle = this.options.backgroundColor;
        this.ctx.fillRect(0, 0, this.width, this.height);

        const total = this.data.reduce((sum, item) => sum + item.value, 0);
        const centerX = this.width / 2;
        const centerY = this.height / 2;
        const radius = Math.min(centerX, centerY) - 60;
        const innerRadius = radius * 0.6;

        let currentAngle = -Math.PI / 2;

        this.data.forEach((item, i) => {
            const sliceAngle = (item.value / total) * Math.PI * 2;
            const color = this.options.colors[i % this.options.colors.length];

            // Draw slice
            this.ctx.fillStyle = color;
            this.ctx.beginPath();
            this.ctx.arc(centerX, centerY, radius, currentAngle, currentAngle + sliceAngle);
            this.ctx.arc(centerX, centerY, innerRadius, currentAngle + sliceAngle, currentAngle, true);
            this.ctx.closePath();
            this.ctx.fill();

            currentAngle += sliceAngle;
        });

        // Draw center circle (creates doughnut hole)
        this.ctx.fillStyle = this.options.backgroundColor;
        this.ctx.beginPath();
        this.ctx.arc(centerX, centerY, innerRadius, 0, Math.PI * 2);
        this.ctx.fill();

        // Draw total in center
        this.ctx.fillStyle = this.options.textColor;
        this.ctx.font = 'bold 24px sans-serif';
        this.ctx.textAlign = 'center';
        this.ctx.textBaseline = 'middle';
        this.ctx.fillText(total, centerX, centerY - 10);
        this.ctx.font = '12px sans-serif';
        this.ctx.fillText('Total', centerX, centerY + 15);

        // Draw legend
        if (this.options.showLegend) {
            this.drawLegend();
        }

        // Draw title
        if (this.options.title) {
            this.ctx.font = 'bold 14px sans-serif';
            this.ctx.textAlign = 'center';
            this.ctx.fillText(this.options.title, this.width / 2, 20);
        }
    }

    drawLegend() {
        const legendX = 20;
        let legendY = this.height - (this.data.length * 25) - 20;

        this.data.forEach((item, i) => {
            const color = this.options.colors[i % this.options.colors.length];

            // Color box
            this.ctx.fillStyle = color;
            this.ctx.fillRect(legendX, legendY, 15, 15);

            // Label
            this.ctx.fillStyle = this.options.textColor;
            this.ctx.font = '12px sans-serif';
            this.ctx.textAlign = 'left';
            this.ctx.fillText(`${item.label}: ${item.value}`, legendX + 20, legendY + 12);

            legendY += 25;
        });
    }

    drawEmptyState() {
        this.ctx.fillStyle = this.options.backgroundColor;
        this.ctx.fillRect(0, 0, this.width, this.height);

        this.ctx.fillStyle = this.options.textColor;
        this.ctx.font = '14px sans-serif';
        this.ctx.textAlign = 'center';
        this.ctx.fillText('No data available', this.width / 2, this.height / 2);
    }
}

/**
 * Bar chart for comparisons
 */
class BarChart {
    constructor(canvasId, options = {}) {
        this.canvas = document.getElementById(canvasId);
        if (!this.canvas) {
            console.error(`Canvas element ${canvasId} not found`);
            return;
        }

        this.ctx = this.canvas.getContext('2d');
        this.options = {
            title: options.title || '',
            barColor: options.barColor || '#f97316',
            gridColor: options.gridColor || '#334155',
            textColor: options.textColor || '#94a3b8',
            backgroundColor: options.backgroundColor || '#1e293b',
            padding: options.padding || 50,
            ...options
        };

        this.data = [];
        this.resize();
        window.addEventListener('resize', () => this.resize());
    }

    resize() {
        const rect = this.canvas.getBoundingClientRect();
        this.canvas.width = rect.width * window.devicePixelRatio;
        this.canvas.height = rect.height * window.devicePixelRatio;
        this.ctx.scale(window.devicePixelRatio, window.devicePixelRatio);
        this.width = rect.width;
        this.height = rect.height;

        if (this.data.length > 0) {
            this.draw();
        }
    }

    setData(data) {
        this.data = data;
        this.draw();
    }

    draw() {
        if (!this.data || this.data.length === 0) {
            this.drawEmptyState();
            return;
        }

        const { padding } = this.options;
        const chartWidth = this.width - (padding * 2);
        const chartHeight = this.height - (padding * 2);

        // Clear canvas
        this.ctx.fillStyle = this.options.backgroundColor;
        this.ctx.fillRect(0, 0, this.width, this.height);

        // Calculate scales
        const maxValue = Math.max(...this.data.map(d => d.value));
        const barWidth = chartWidth / this.data.length;
        const barPadding = barWidth * 0.2;

        // Draw grid
        this.drawGrid(padding, chartWidth, chartHeight, maxValue);

        // Draw bars
        this.data.forEach((item, i) => {
            const barHeight = (item.value / maxValue) * chartHeight;
            const x = padding + (i * barWidth) + barPadding;
            const y = padding + chartHeight - barHeight;
            const width = barWidth - (barPadding * 2);

            // Bar
            this.ctx.fillStyle = this.options.barColor;
            this.ctx.fillRect(x, y, width, barHeight);

            // Label
            this.ctx.fillStyle = this.options.textColor;
            this.ctx.font = '12px sans-serif';
            this.ctx.textAlign = 'center';
            this.ctx.fillText(item.label, x + width / 2, this.height - padding + 20);

            // Value on top
            this.ctx.fillText(item.value, x + width / 2, y - 5);
        });

        // Title
        if (this.options.title) {
            this.ctx.font = 'bold 14px sans-serif';
            this.ctx.textAlign = 'center';
            this.ctx.fillText(this.options.title, this.width / 2, 20);
        }
    }

    drawGrid(padding, chartWidth, chartHeight, maxValue) {
        this.ctx.strokeStyle = this.options.gridColor;
        this.ctx.lineWidth = 1;
        this.ctx.setLineDash([5, 5]);

        const gridLines = 5;
        for (let i = 0; i <= gridLines; i++) {
            const y = padding + (chartHeight / gridLines) * i;
            this.ctx.beginPath();
            this.ctx.moveTo(padding, y);
            this.ctx.lineTo(padding + chartWidth, y);
            this.ctx.stroke();

            const value = maxValue - (maxValue / gridLines) * i;
            this.ctx.fillStyle = this.options.textColor;
            this.ctx.font = '12px sans-serif';
            this.ctx.textAlign = 'right';
            this.ctx.fillText(Math.round(value), padding - 10, y + 4);
        }

        this.ctx.setLineDash([]);
    }

    drawEmptyState() {
        this.ctx.fillStyle = this.options.backgroundColor;
        this.ctx.fillRect(0, 0, this.width, this.height);

        this.ctx.fillStyle = this.options.textColor;
        this.ctx.font = '14px sans-serif';
        this.ctx.textAlign = 'center';
        this.ctx.fillText('No data available', this.width / 2, this.height / 2);
    }
}
