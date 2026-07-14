export function StatusBadge({ status }: { status: string }) {
  const cls =
    status === 'succeeded'
      ? 'badge-succeeded'
      : status === 'failed'
        ? 'badge-failed'
        : status === 'running'
          ? 'badge-running'
          : 'badge-pending'
  return <span className={`badge ${cls}`}>{status || 'unknown'}</span>
}

export function formatTime(iso: string) {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}
