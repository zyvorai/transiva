# HyperSDK Helm Charts Repository

This directory contains packaged Helm charts for HyperSDK, published via GitHub Pages.

## Repository URL

```
https://ssahani.github.io/transiva/helm-charts
```

## Usage

Add the repository:
```bash
helm repo add transiva https://ssahani.github.io/transiva/helm-charts
helm repo update
```

Install a chart:
```bash
helm install transiva transiva/transiva
```

## Available Charts

- **transiva** - Multi-cloud VM export and migration toolkit

## Files

- `index.yaml` - Helm repository index
- `index.html` - Web interface for the repository
- `*.tgz` - Packaged Helm charts

## Automation

This directory is automatically updated by the chart packaging and release workflows.

Do not manually edit files in this directory.
