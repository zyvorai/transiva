// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { AlertsList } from './AlertsList'
import type { Alert } from '../types/metrics'

const createMockAlert = (overrides?: Partial<Alert>): Alert => ({
  id: 'alert-1',
  title: 'High Memory Usage',
  message: 'Memory usage is above 80%',
  severity: 'warning',
  timestamp: '2026-01-21T10:00:00Z',
  acknowledged: false,
  ...overrides,
})

describe('AlertsList', () => {
  it('renders "No active alerts" when alerts array is empty', () => {
    render(<AlertsList alerts={[]} />)

    expect(screen.getByText('No active alerts')).toBeDefined()
  })

  it('renders single alert correctly', () => {
    const alert = createMockAlert()
    render(<AlertsList alerts={[alert]} />)

    expect(screen.getByText('High Memory Usage')).toBeDefined()
    expect(screen.getByText('Memory usage is above 80%')).toBeDefined()
    expect(screen.getByText('warning')).toBeDefined()
  })

  it('renders multiple alerts', () => {
    const alerts = [
      createMockAlert({ id: 'alert-1', title: 'Alert 1' }),
      createMockAlert({ id: 'alert-2', title: 'Alert 2' }),
      createMockAlert({ id: 'alert-3', title: 'Alert 3' }),
    ]
    render(<AlertsList alerts={alerts} />)

    expect(screen.getByText('Alert 1')).toBeDefined()
    expect(screen.getByText('Alert 2')).toBeDefined()
    expect(screen.getByText('Alert 3')).toBeDefined()
  })

  it('renders different severity levels correctly', () => {
    const alerts = [
      createMockAlert({ id: 'info', severity: 'info', title: 'Info Alert' }),
      createMockAlert({ id: 'warning', severity: 'warning', title: 'Warning Alert' }),
      createMockAlert({ id: 'error', severity: 'error', title: 'Error Alert' }),
      createMockAlert({ id: 'critical', severity: 'critical', title: 'Critical Alert' }),
    ]
    render(<AlertsList alerts={alerts} />)

    expect(screen.getByText('info')).toBeDefined()
    expect(screen.getByText('warning')).toBeDefined()
    expect(screen.getByText('error')).toBeDefined()
    expect(screen.getByText('critical')).toBeDefined()
  })

  it('renders dismiss button when onDismiss is provided and alert is not acknowledged', () => {
    const onDismiss = vi.fn()
    const alert = createMockAlert({ acknowledged: false })

    render(<AlertsList alerts={[alert]} onDismiss={onDismiss} />)

    expect(screen.getByText('Dismiss')).toBeDefined()
  })

  it('does not render dismiss button when alert is acknowledged', () => {
    const onDismiss = vi.fn()
    const alert = createMockAlert({ acknowledged: true })

    render(<AlertsList alerts={[alert]} onDismiss={onDismiss} />)

    expect(screen.queryByText('Dismiss')).toBeNull()
  })

  it('does not render dismiss button when onDismiss is not provided', () => {
    const alert = createMockAlert({ acknowledged: false })

    render(<AlertsList alerts={[alert]} />)

    expect(screen.queryByText('Dismiss')).toBeNull()
  })

  it('calls onDismiss with alert id when dismiss button is clicked', () => {
    const onDismiss = vi.fn()
    const alert = createMockAlert({ id: 'test-alert-123' })

    render(<AlertsList alerts={[alert]} onDismiss={onDismiss} />)

    const dismissButton = screen.getByText('Dismiss')
    fireEvent.click(dismissButton)

    expect(onDismiss).toHaveBeenCalledWith('test-alert-123')
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('handles multiple dismiss actions correctly', () => {
    const onDismiss = vi.fn()
    const alerts = [
      createMockAlert({ id: 'alert-1', title: 'Alert 1', acknowledged: false }),
      createMockAlert({ id: 'alert-2', title: 'Alert 2', acknowledged: false }),
    ]

    render(<AlertsList alerts={alerts} onDismiss={onDismiss} />)

    const dismissButtons = screen.getAllByText('Dismiss')
    expect(dismissButtons).toHaveLength(2)

    fireEvent.click(dismissButtons[0])
    expect(onDismiss).toHaveBeenCalledWith('alert-1')

    fireEvent.click(dismissButtons[1])
    expect(onDismiss).toHaveBeenCalledWith('alert-2')

    expect(onDismiss).toHaveBeenCalledTimes(2)
  })

  it('renders alert with all properties', () => {
    const alert = createMockAlert({
      id: 'complete-alert',
      title: 'Complete Alert',
      message: 'This is a complete alert with all properties',
      severity: 'error',
      timestamp: '2026-01-21T10:00:00Z',
      acknowledged: false,
    })

    render(<AlertsList alerts={[alert]} onDismiss={vi.fn()} />)

    expect(screen.getByText('Complete Alert')).toBeDefined()
    expect(screen.getByText('This is a complete alert with all properties')).toBeDefined()
    expect(screen.getByText('error')).toBeDefined()
    expect(screen.getByText('Dismiss')).toBeDefined()
  })

  it('renders alerts in the order they are provided', () => {
    const alerts = [
      createMockAlert({ id: '1', title: 'First' }),
      createMockAlert({ id: '2', title: 'Second' }),
      createMockAlert({ id: '3', title: 'Third' }),
    ]

    const { container } = render(<AlertsList alerts={alerts} />)
    const titles = screen.getAllByText(/First|Second|Third/)

    expect(titles[0].textContent).toBe('First')
    expect(titles[1].textContent).toBe('Second')
    expect(titles[2].textContent).toBe('Third')
  })
})
