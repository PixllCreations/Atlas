export function DeploymentFailure({ failure }: { failure?: string }) {
  if (!failure) return null
  return <div role="alert"><h3>Deployment failed</h3><pre className="build-log" style={{ whiteSpace: 'pre-wrap' }}>{failure}</pre></div>
}
