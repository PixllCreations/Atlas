import { BUILD_PHASES, type Build, type BuildPhase } from '../api/client'

export function phaseIndex(phase: BuildPhase | undefined): number {
  if (!phase) return -1
  return BUILD_PHASES.findIndex((p) => p.id === phase)
}

export type PhaseStepState = 'done' | 'active' | 'failed' | 'upcoming'

/** Progress stays at the last reached step: prior steps succeeded, current failed/active, rest upcoming. */
export function phaseStepState(build: Build, index: number): PhaseStepState {
  const idx = phaseIndex(build.phase)
  if (build.status === 'succeeded') return 'done'
  if (build.status === 'failed') {
    if (idx < 0) return index === 0 ? 'failed' : 'upcoming'
    if (index < idx) return 'done'
    if (index === idx) return 'failed'
    return 'upcoming'
  }
  // pending / running
  if (idx < 0) return index === 0 ? 'active' : 'upcoming'
  if (index < idx) return 'done'
  if (index === idx) return 'active'
  return 'upcoming'
}

type Props = {
  build: Build
  compact?: boolean
  elapsedSeconds?: number | null
}

export function BuildPhases({ build, compact = false, elapsedSeconds = null }: Props) {
  return (
    <div className={compact ? 'build-phases build-phases-compact' : 'build-phases'}>
      <div className={compact ? 'phase-steps phase-steps-compact' : 'phase-steps'}>
        {BUILD_PHASES.map((p, i) => {
          const state = phaseStepState(build, i)
          return (
            <div
              key={p.id}
              className={`phase-step phase-${state}${compact ? ' phase-compact' : ''}`}
              title={p.label}
            >
              <span className="phase-dot" />
              <span className={compact ? 'phase-compact-label' : undefined}>{p.label}</span>
            </div>
          )
        })}
      </div>
      {!compact && elapsedSeconds != null ? (
        <p className="phase-elapsed muted">
          Elapsed {elapsedSeconds}s · <span className="mono">{build.phase || 'queued'}</span>
        </p>
      ) : null}
    </div>
  )
}
