// k6-operator TestRun stages are: initialization, initialized, created,
// started, stopped, finished and error. The backend reports whether the
// runners are done (`finished`), whether the test was stopped through
// k6-manager (`stoppedAt`) and, when it can tell from the runner Jobs, whether
// the run passed or failed (`result`). A run whose k6 thresholds fail still
// reaches the "finished" stage, so the stage alone doesn't mean success.

const ACTIVE_LABELS = {
  initialization: 'Initializing',
  initialized: 'Initialized',
  created: 'Starting',
  started: 'Running',
};

export const STATUS_FILTERS = [
  { value: 'all', label: 'All Statuses' },
  { value: 'active', label: 'Running' },
  { value: 'passed', label: 'Passed' },
  { value: 'failed', label: 'Failed' },
  { value: 'stopped', label: 'Stopped' },
  { value: 'finished', label: 'Finished (no result)' },
];

// testStatus maps a test to a display category and label. Every test falls
// into exactly one of the STATUS_FILTERS categories.
export function testStatus(test) {
  const phase = test?.phase?.toLowerCase() ?? '';

  if (!test?.finished) {
    return { category: 'active', label: ACTIVE_LABELS[phase] ?? (phase || 'Pending') };
  }
  if (phase === 'error') {
    return { category: 'failed', label: 'Error', detail: 'k6-operator reported an error running this test.' };
  }
  if (test.stoppedAt) {
    return { category: 'stopped', label: 'Stopped', detail: 'This test was stopped before it completed.' };
  }
  if (test.result === 'failed') {
    return {
      category: 'failed',
      label: 'Failed',
      detail: 'At least one runner exited with an error, for example because thresholds were crossed.',
    };
  }
  if (test.result === 'passed') {
    return { category: 'passed', label: 'Passed', detail: 'All runners completed successfully.' };
  }
  return {
    category: 'finished',
    label: 'Finished',
    detail: 'The test finished, but its result could not be determined from the runner Jobs.',
  };
}

// errorMessage extracts a human-readable message from an API error, preferring
// field-level validation errors over the generic problem detail.
export function errorMessage(err, fallback) {
  const data = err?.response?.data;
  if (data?.errors && Object.keys(data.errors).length > 0) {
    return Object.entries(data.errors)
      .map(([field, msgs]) => `${field}: ${msgs.join(', ')}`)
      .join(' | ');
  }
  return data?.detail || fallback;
}
