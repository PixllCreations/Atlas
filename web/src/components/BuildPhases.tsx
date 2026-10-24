import { BUILD_PHASES, type Build } from '../api/client'


import { phaseStepState } from './buildPhaseState'
export { phaseStepState, phaseIndex } from './buildPhaseState'

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
              title={`${p.label}: ${state === 'done' ? 'Succeeded' : state === 'failed' ? 'Failed' : state === 'active' ? 'In progress' : 'Pending'}`}
              aria-label={`${p.label}: ${state === 'done' ? 'Succeeded' : state === 'failed' ? 'Failed' : state === 'active' ? 'In progress' : 'Pending'}`}
            >
              <span className="phase-dot" />
              <span className={compact ? 'phase-compact-label' : undefined}>{p.label}{!compact ? ` · ${state === 'done' ? 'Succeeded' : state === 'failed' ? 'Failed' : state === 'active' ? 'In progress' : 'Pending'}` : ''}</span>
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
