# HyperSDK React Dashboard

Modern React/TypeScript dashboard for HyperSDK with real-time WebSocket updates and advanced visualizations.

## Features

- **Real-time Monitoring**: WebSocket-based live metrics updates
- **Interactive Charts**: Recharts-powered visualizations with historical data
- **Job Management**: View, filter, and manage export jobs
- **Export Workflows**: vSphere and Nutanix AHV guided export wizards
- **Alert System**: Real-time alerts and notifications
- **Provider Analytics**: Multi-cloud provider comparison
- **Responsive Design**: Works on desktop and mobile devices

## Development

### Prerequisites

- Node.js 18+
- npm or yarn

### Install Dependencies

```bash
cd web/dashboard-react
npm install
```

### Development Server

Run the development server with hot module replacement:

```bash
npm run dev
```

This will start the Vite dev server at `http://localhost:5173` with proxy configuration forwarding API calls to `http://localhost:8080`.

Make sure the HyperSDK daemon is running:

```bash
# In the project root
./bin/hypervisord --config config.yaml
```

### Build for Production

Build the React app for production:

```bash
npm run build
```

This will compile TypeScript, bundle assets, and output the production build to `daemon/dashboard/static-react/`.

### Preview Production Build

Preview the production build locally:

```bash
npm run preview
```

## Project Structure

```
web/dashboard-react/
├── src/
│   ├── components/          # React components
│   │   ├── Dashboard.tsx    # Main dashboard layout
│   │   ├── ExportWorkflowRouter.tsx  # vSphere / Nutanix export tabs
│   │   ├── NutanixExportWorkflow.tsx # Nutanix discovery + export wizard
│   │   ├── StatCard.tsx     # Metric card component
│   │   ├── JobsTable.tsx    # Jobs table with sorting/filtering
│   │   ├── AlertsList.tsx   # Alerts display
│   │   └── ChartContainer.tsx # Chart wrapper
│   ├── hooks/               # Custom React hooks
│   │   ├── useWebSocket.ts  # WebSocket connection hook
│   │   └── useMetricsHistory.ts # Metrics history management
│   ├── types/               # TypeScript type definitions
│   │   └── metrics.ts       # API types and interfaces
│   ├── utils/               # Utility functions
│   │   ├── api.ts           # API client functions
│   │   └── formatters.ts    # Formatting utilities
│   ├── App.tsx              # Root component
│   └── main.tsx             # Entry point
├── public/                  # Static assets
├── index.html               # HTML template
├── package.json             # Dependencies and scripts
├── tsconfig.json            # TypeScript configuration
├── vite.config.ts           # Vite build configuration
└── README.md                # This file
```

## Technology Stack

- **React 18**: UI library
- **TypeScript**: Type-safe JavaScript
- **Vite**: Build tool and dev server
- **Recharts**: Charting library
- **Zustand**: State management (if needed)
- **React Query**: Data fetching and caching

## Key Components

### Dashboard

Main dashboard component that orchestrates all sub-components and manages WebSocket connection.

**Features**:
- Real-time metrics display
- Job activity charts
- Resource usage monitoring
- Provider comparison
- Recent jobs table

### useWebSocket Hook

Custom hook for WebSocket connection management with automatic reconnection.

**Features**:
- Automatic reconnection with backoff
- Connection state tracking
- Error handling
- Message sending

### ChartContainer

Reusable chart component supporting multiple chart types.

**Supported Charts**:
- Line charts (time series)
- Bar charts (comparisons)
- Pie charts (distributions)

## API Integration

The dashboard connects to the HyperSDK API at the following endpoints:

- `GET /health` - Health check
- `GET /status` - System status
- `GET /capabilities` - Provider capabilities
- `GET /providers/list` - Registered providers
- `GET /providers/capabilities?provider=nutanix` - Nutanix export formats
- `GET /vms/list?provider=nutanix` - List Nutanix VMs
- `POST /vms/export?provider=nutanix` - Synchronous Nutanix export
- `POST /jobs/submit` - Submit new job (use `provider: nutanix` for async export)
- `GET /jobs/query` - Query jobs
- `POST /jobs/cancel` - Cancel job
- `GET /schedules` - List schedules
- `POST /schedules` - Create schedule
- `GET /webhooks` - List webhooks
- `WS /ws` - WebSocket real-time updates

## Customization

### Styling

The dashboard uses inline styles for simplicity. To customize:

1. Modify color constants in components
2. Adjust layout grid configurations
3. Update chart colors in `formatters.ts`

### Adding New Metrics

1. Update `types/metrics.ts` with new metric fields
2. Add display logic in `Dashboard.tsx`
3. Create new StatCard or chart as needed

### Adding New Charts

1. Use `ChartContainer` component
2. Transform metrics data to chart format
3. Add to dashboard layout

## Deployment

### Automated (Recommended)

The build script automatically builds the React dashboard:

```bash
# From project root
./build.sh
```

This will:
1. Build the React app
2. Copy output to `daemon/dashboard/static-react/`
3. Build Go binaries
4. Create distribution package

### Manual

```bash
# Build React app
cd web/dashboard-react
npm run build

# Build Go daemon
cd ../..
go build -o bin/hypervisord ./cmd/hypervisord

# Run daemon
./bin/hypervisord --config config.yaml

# Access dashboard at http://localhost:8080/web/dashboard/
```

## Backward Compatibility

The React dashboard replaces the vanilla JavaScript dashboard at `/web/dashboard/`.

The legacy dashboard remains available at `/web/dashboard-legacy/` for backward compatibility.

To disable web dashboards entirely and run API-only mode:

```yaml
# config.yaml
web:
  disabled: true
```

## Performance

- **Initial Load**: < 500KB gzipped bundle
- **WebSocket Updates**: < 100ms latency
- **Chart Rendering**: 60 FPS with 100+ data points
- **Memory Usage**: ~50MB JavaScript heap

## Browser Support

- Chrome/Edge 90+
- Firefox 88+
- Safari 14+

## Troubleshooting

### WebSocket Connection Fails

Check that:
1. HyperSDK daemon is running
2. WebSocket endpoint is accessible at `/ws`
3. CORS is configured correctly
4. No proxy/firewall blocking WebSocket connections

### Build Errors

```bash
# Clean install
rm -rf node_modules package-lock.json
npm install

# Clear Vite cache
rm -rf node_modules/.vite
npm run build
```

### Chart Not Rendering

Ensure:
1. Data format matches expected structure
2. `dataKeys` prop matches data fields
3. Browser supports SVG rendering

## License

Apache-2.0 (same as HyperSDK)

## Contributing

1. Make changes in `src/`
2. Test with `npm run dev`
3. Build with `npm run build`
4. Verify production build works
5. Submit pull request

## Support

- Documentation: [../../DEPLOYMENT.md](../../DEPLOYMENT.md)
- Issues: https://github.com/your-org/transiva/issues
