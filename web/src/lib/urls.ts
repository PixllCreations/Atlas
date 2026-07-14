export function appPublicURL(appName: string, ingressDomain: string): string {
  const host = `${appName}.${ingressDomain}`
  const secure = !ingressDomain.endsWith('.local')
  return `${secure ? 'https' : 'http'}://${host}`
}
