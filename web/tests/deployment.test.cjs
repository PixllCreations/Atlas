const { test, after } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const ts = require('typescript')
const { renderToStaticMarkup } = require('react-dom/server')
const React = require('react')

// Compile the real components without requiring a browser or additional test dependencies.
fs.mkdirSync(path.join(__dirname, '../node_modules/.tmp'), { recursive: true })
const output = fs.mkdtempSync(path.join(__dirname, '../node_modules/.tmp/deployment-test-'))
fs.writeFileSync(path.join(output, 'package.json'), '{"type":"commonjs"}')
for (const file of ['api/client.ts', 'components/buildPhaseState.ts', 'components/BuildPhases.tsx', 'components/DeploymentFailure.tsx']) {
  const source = fs.readFileSync(path.join(__dirname, '../src', file), 'utf8')
  const result = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2023 } })
  const target = path.join(output, file.replace(/\.tsx?$/, '.js'))
  fs.mkdirSync(path.dirname(target), { recursive: true })
  fs.writeFileSync(target, result.outputText)
}
after(() => fs.rmSync(output, { recursive: true, force: true }))
const { phaseStepState } = require(path.join(output, 'components/buildPhaseState.js'))
const { BuildPhases } = require(path.join(output, 'components/BuildPhases.js'))
const { DeploymentFailure } = require(path.join(output, 'components/DeploymentFailure.js'))
const build = { id: 'b', app_id: 'a', image: 'new-image', phase: 'deploying', status: 'failed', created_at: '', updated_at: '' }

test('runtime failure preserves building and pushing successes', () => {
  assert.deepEqual([0, 1, 2, 3, 4].map(i => phaseStepState(build, i)), ['done', 'done', 'done', 'done', 'failed'])
  const html = renderToStaticMarkup(React.createElement(BuildPhases, { build }))
  for (const label of ['Building: Succeeded', 'Pushing: Succeeded', 'Deploying: Failed']) assert.ok(html.includes(label))
})
test('applying resources stays active until rollout succeeds', () => {
  assert.equal(phaseStepState({ ...build, status: 'running' }, 4), 'active')
  assert.equal(phaseStepState({ ...build, status: 'succeeded' }, 4), 'done')
  assert.equal(phaseStepState({ ...build, phase: 'building', status: 'succeeded' }, 4), 'upcoming')
})
test('failure panel displays persisted diagnostics safely', () => {
  const failure = 'New application revision did not become ready.\nPod: app-new\nExit code: 1\nPrevious revision remains available.\n<script>alert(1)</script>'
  const html = renderToStaticMarkup(React.createElement(DeploymentFailure, { failure }))
  assert.ok(html.includes('role="alert"'))
  assert.ok(html.includes('Exit code: 1'))
  assert.ok(html.includes('Previous revision remains available'))
  assert.ok(!html.includes('<script>'))
  assert.equal(renderToStaticMarkup(React.createElement(DeploymentFailure, {})), '')
})
