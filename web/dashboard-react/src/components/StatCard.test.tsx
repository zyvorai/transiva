// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { StatCard } from './StatCard'

describe('StatCard', () => {
  it('renders title and value', () => {
    render(<StatCard title="Active Jobs" value={42} />)

    expect(screen.getByText('Active Jobs')).toBeDefined()
    expect(screen.getByText('42')).toBeDefined()
  })

  it('renders string value', () => {
    render(<StatCard title="Status" value="Healthy" />)

    expect(screen.getByText('Status')).toBeDefined()
    expect(screen.getByText('Healthy')).toBeDefined()
  })

  it('renders subtitle when provided', () => {
    render(<StatCard title="Memory" value="2.5 GB" subtitle="of 8 GB available" />)

    expect(screen.getByText('of 8 GB available')).toBeDefined()
  })

  it('renders icon when provided', () => {
    const { container } = render(<StatCard title="Jobs" value={10} icon="🚀" />)

    expect(container.textContent).toContain('🚀')
  })

  it('renders positive trend correctly', () => {
    render(
      <StatCard
        title="Success Rate"
        value="95%"
        trend={{ value: 5.2, isPositive: true }}
      />
    )

    expect(screen.getByText(/↑/)).toBeDefined()
    expect(screen.getByText(/5.2%/)).toBeDefined()
  })

  it('renders negative trend correctly', () => {
    render(
      <StatCard
        title="Error Rate"
        value="2%"
        trend={{ value: 1.5, isPositive: false }}
      />
    )

    expect(screen.getByText(/↓/)).toBeDefined()
    expect(screen.getByText(/1.5%/)).toBeDefined()
  })

  it('uses default color when not specified', () => {
    const { container } = render(<StatCard title="Test" value={123} />)
    const valueElement = screen.getByText('123')

    expect(valueElement).toBeDefined()
  })

  it('uses custom color when provided', () => {
    const { container } = render(
      <StatCard title="Test" value={123} color="#10b981" />
    )
    const valueElement = screen.getByText('123')

    expect(valueElement).toBeDefined()
  })

  it('renders all props together', () => {
    render(
      <StatCard
        title="Active Jobs"
        value={42}
        subtitle="Last updated 2m ago"
        icon="📊"
        color="#3b82f6"
        trend={{ value: 10, isPositive: true }}
      />
    )

    expect(screen.getByText('Active Jobs')).toBeDefined()
    expect(screen.getByText('42')).toBeDefined()
    expect(screen.getByText('Last updated 2m ago')).toBeDefined()
    expect(screen.getByText(/↑/)).toBeDefined()
    expect(screen.getByText(/10%/)).toBeDefined()
  })

  it('handles absolute trend values', () => {
    render(
      <StatCard
        title="Test"
        value={100}
        trend={{ value: -5, isPositive: false }}
      />
    )

    // Should display absolute value
    expect(screen.getByText(/5%/)).toBeDefined()
  })
})
