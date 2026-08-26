// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { JobsTable } from './JobsTable'
import type { JobInfo } from '../types/metrics'

const createMockJob = (overrides?: Partial<JobInfo>): JobInfo => ({
  id: 'job-123',
  name: 'Export VM',
  status: 'running',
  progress: 50,
  start_time: '2026-01-21T10:00:00Z',
  end_time: undefined,
  duration_seconds: 300,
  provider: 'vsphere',
  vm_name: 'test-vm',
  format: 'ova',
  compress: false,
  error_msg: undefined,
  ...overrides,
})

describe('JobsTable', () => {
  it('renders "No jobs found" when jobs array is empty', () => {
    render(<JobsTable jobs={[]} />)

    expect(screen.getByText('No jobs found')).toBeDefined()
  })

  it('renders single job correctly', () => {
    const job = createMockJob({ name: 'Test Job', vm_name: 'my-vm' })
    render(<JobsTable jobs={[job]} />)

    expect(screen.getByText('Test Job')).toBeDefined()
    expect(screen.getByText('my-vm')).toBeDefined()
  })

  it('renders multiple jobs', () => {
    const jobs = [
      createMockJob({ id: '1', name: 'Job 1', vm_name: 'vm-1' }),
      createMockJob({ id: '2', name: 'Job 2', vm_name: 'vm-2' }),
      createMockJob({ id: '3', name: 'Job 3', vm_name: 'vm-3' }),
    ]
    render(<JobsTable jobs={jobs} />)

    expect(screen.getByText('Job 1')).toBeDefined()
    expect(screen.getByText('Job 2')).toBeDefined()
    expect(screen.getByText('Job 3')).toBeDefined()
  })

  it('renders job status with icon', () => {
    const job = createMockJob({ status: 'completed' })
    const { container } = render(<JobsTable jobs={[job]} />)

    // Check for status badge (not the filter dropdown)
    const statusBadges = container.querySelectorAll('span[style*="inline-flex"]')
    const completedBadge = Array.from(statusBadges).find(el =>
      el.textContent?.includes('completed')
    )
    expect(completedBadge).toBeDefined()
  })

  it('renders progress bar', () => {
    const job = createMockJob({ progress: 75 })
    const { container } = render(<JobsTable jobs={[job]} />)

    expect(screen.getByText('75%')).toBeDefined()
  })

  it('renders provider information', () => {
    const job = createMockJob({ provider: 'aws' })
    render(<JobsTable jobs={[job]} />)

    expect(screen.getByText('aws')).toBeDefined()
  })

  it('renders format and compression info', () => {
    const job = createMockJob({ format: 'ova', compress: true })
    const { container } = render(<JobsTable jobs={[job]} />)

    // Check container for format and compression
    expect(container.textContent).toContain('OVA')
    expect(container.textContent).toContain('Compressed')
  })

  it('renders format without compression', () => {
    const job = createMockJob({ format: 'vmdk', compress: false })
    const { container } = render(<JobsTable jobs={[job]} />)

    expect(container.textContent).toContain('VMDK')
    expect(container.textContent).not.toContain('Compressed')
  })

  it('shows cancel button for running jobs when onCancelJob is provided', () => {
    const onCancelJob = vi.fn()
    const job = createMockJob({ status: 'running' })

    render(<JobsTable jobs={[job]} onCancelJob={onCancelJob} />)

    expect(screen.getByText('Cancel')).toBeDefined()
  })

  it('does not show cancel button for completed jobs', () => {
    const onCancelJob = vi.fn()
    const job = createMockJob({ status: 'completed' })

    render(<JobsTable jobs={[job]} onCancelJob={onCancelJob} />)

    expect(screen.queryByText('Cancel')).toBeNull()
  })

  it('does not show cancel button when onCancelJob is not provided', () => {
    const job = createMockJob({ status: 'running' })

    render(<JobsTable jobs={[job]} />)

    expect(screen.queryByText('Cancel')).toBeNull()
  })

  it('calls onCancelJob with job id when cancel button is clicked', () => {
    const onCancelJob = vi.fn()
    const job = createMockJob({ id: 'job-456', status: 'running' })

    render(<JobsTable jobs={[job]} onCancelJob={onCancelJob} />)

    const cancelButton = screen.getByText('Cancel')
    fireEvent.click(cancelButton)

    expect(onCancelJob).toHaveBeenCalledWith('job-456')
    expect(onCancelJob).toHaveBeenCalledTimes(1)
  })

  it('filters jobs by status', () => {
    const jobs = [
      createMockJob({ id: '1', name: 'Running Job', status: 'running' }),
      createMockJob({ id: '2', name: 'Completed Job', status: 'completed' }),
      createMockJob({ id: '3', name: 'Failed Job', status: 'failed' }),
    ]

    render(<JobsTable jobs={jobs} />)

    // Change filter to 'completed'
    const filterSelect = screen.getByRole('combobox')
    fireEvent.change(filterSelect, { target: { value: 'completed' } })

    expect(screen.getByText('Completed Job')).toBeDefined()
    expect(screen.queryByText('Running Job')).toBeNull()
    expect(screen.queryByText('Failed Job')).toBeNull()
  })

  it('shows all jobs when filter is set to "all"', () => {
    const jobs = [
      createMockJob({ id: '1', name: 'Running Job', status: 'running' }),
      createMockJob({ id: '2', name: 'Completed Job', status: 'completed' }),
    ]

    render(<JobsTable jobs={jobs} />)

    const filterSelect = screen.getByRole('combobox')
    fireEvent.change(filterSelect, { target: { value: 'all' } })

    expect(screen.getByText('Running Job')).toBeDefined()
    expect(screen.getByText('Completed Job')).toBeDefined()
  })

  it('sorts jobs by name in ascending order', () => {
    const jobs = [
      createMockJob({ id: '1', name: 'Charlie' }),
      createMockJob({ id: '2', name: 'Alice' }),
      createMockJob({ id: '3', name: 'Bob' }),
    ]

    const { container } = render(<JobsTable jobs={jobs} />)

    // Click on Name header to sort
    const nameHeader = screen.getByText(/Name/)
    fireEvent.click(nameHeader)

    const jobNames = screen.getAllByText(/Alice|Bob|Charlie/)
    expect(jobNames[0].textContent).toBe('Alice')
    expect(jobNames[1].textContent).toBe('Bob')
    expect(jobNames[2].textContent).toBe('Charlie')
  })

  it('toggles sort direction when clicking same header twice', () => {
    const jobs = [
      createMockJob({ id: '1', name: 'Alice' }),
      createMockJob({ id: '2', name: 'Bob' }),
    ]

    render(<JobsTable jobs={jobs} />)

    const nameHeader = screen.getByText(/Name/)

    // First click - ascending
    fireEvent.click(nameHeader)
    expect(screen.getByText(/Name.*↑/)).toBeDefined()

    // Second click - descending
    fireEvent.click(nameHeader)
    expect(screen.getByText(/Name.*↓/)).toBeDefined()
  })

  it('sorts jobs by progress numerically', () => {
    const jobs = [
      createMockJob({ id: '1', vm_name: 'vm-1', progress: 75 }),
      createMockJob({ id: '2', vm_name: 'vm-2', progress: 25 }),
      createMockJob({ id: '3', vm_name: 'vm-3', progress: 50 }),
    ]

    const { container } = render(<JobsTable jobs={jobs} />)

    const progressHeader = screen.getByText(/Progress/)
    fireEvent.click(progressHeader)

    // Check that jobs are sorted correctly by checking vm names in order
    const vmNames = screen.getAllByText(/vm-[123]/)
    expect(vmNames[0].textContent).toBe('vm-2') // 25%
    expect(vmNames[1].textContent).toBe('vm-3') // 50%
    expect(vmNames[2].textContent).toBe('vm-1') // 75%
  })

  it('handles jobs with missing optional fields', () => {
    const job = createMockJob({
      format: undefined,
      compress: undefined,
      provider: undefined,
    })

    render(<JobsTable jobs={[job]} />)

    expect(screen.getByText('N/A')).toBeDefined()
  })

  it('truncates and displays job ID', () => {
    const job = createMockJob({ id: '12345678-1234-1234-1234-123456789012' })
    render(<JobsTable jobs={[job]} />)

    // Should show first 8 characters
    expect(screen.getByText('12345678')).toBeDefined()
  })

  it('renders all filter options', () => {
    render(<JobsTable jobs={[]} />)

    const filterSelect = screen.getByRole('combobox')
    const options = Array.from(filterSelect.querySelectorAll('option'))

    expect(options).toHaveLength(6)
    expect(options.map(o => o.value)).toEqual([
      'all',
      'pending',
      'running',
      'completed',
      'failed',
      'cancelled',
    ])
  })

  it('handles complex sorting and filtering together', () => {
    const jobs = [
      createMockJob({ id: '1', name: 'Job A', vm_name: 'vm-a', status: 'running', progress: 30 }),
      createMockJob({ id: '2', name: 'Job B', vm_name: 'vm-b', status: 'running', progress: 70 }),
      createMockJob({ id: '3', name: 'Job C', vm_name: 'vm-c', status: 'completed', progress: 100 }),
    ]

    render(<JobsTable jobs={jobs} />)

    // Filter to running jobs
    const filterSelect = screen.getByRole('combobox')
    fireEvent.change(filterSelect, { target: { value: 'running' } })

    // Should only show 2 running jobs
    expect(screen.getByText('Job A')).toBeDefined()
    expect(screen.getByText('Job B')).toBeDefined()
    expect(screen.queryByText('Job C')).toBeNull()

    // Sort by progress
    const progressHeader = screen.getByText(/Progress/)
    fireEvent.click(progressHeader)

    // Should be sorted by VM name: vm-a (30%), vm-b (70%)
    const vmNames = screen.getAllByText(/vm-[ab]/)
    expect(vmNames[0].textContent).toBe('vm-a')
    expect(vmNames[1].textContent).toBe('vm-b')
  })
})
