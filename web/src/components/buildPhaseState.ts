import { BUILD_PHASES, type Build, type BuildPhase } from '../api/client'

export function phaseIndex(phase: BuildPhase | undefined): number {
  if (!phase) return -1
  return BUILD_PHASES.findIndex((p) => p.id === phase)
}

export type PhaseStepState = 'done' | 'active' | 'failed' | 'upcoming'

/** Progress stays at the last reached step: prior steps succeeded, current failed/active, rest upcoming. */
export function phaseStepState(build: Build, index: number): PhaseStepState {
  const idx = phaseIndex(build.phase)
  if (build.status === 'succeeded') return index <= idx ? 'done' : 'upcoming'
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
