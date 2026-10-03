// Runs a Go action. On the dist branch the prebuilt binary from bin/ is used.
// Everywhere else (main, pull requests, local checkouts) cmd/toolkit is built
// from source, which needs Go on the runner.

const { spawnSync } = require('node:child_process')
const { createHash } = require('node:crypto')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const goos = { linux: 'linux', darwin: 'darwin', win32: 'windows' }[process.platform]
const goarch = { x64: 'amd64', arm64: 'arm64' }[process.arch]
const exe = process.platform === 'win32' ? '.exe' : ''

exports.run = function run(action) {
  const bin = prebuilt() ?? build()
  const res = spawnSync(bin, [action], { stdio: 'inherit' })
  if (res.error) fail(`${action}: ${res.error.message}`)
  if (res.signal) fail(`${action}: killed by ${res.signal}`)
  process.exitCode = res.status
}

function prebuilt() {
  const bin = path.join(root, 'bin', `toolkit-${goos}-${goarch}${exe}`)
  return goos && goarch && fs.existsSync(bin) ? bin : null
}

function build() {
  // Keyed by checkout so actions from different refs in one job don't collide.
  const id = createHash('sha256').update(root).digest('hex').slice(0, 12)
  const out = path.join(process.env.RUNNER_TEMP || os.tmpdir(), `toolkit-${id}`, `toolkit${exe}`)

  console.log('::group::Build toolkit from source')
  const res = spawnSync('go', ['build', '-o', out, './cmd/toolkit'], { cwd: root, stdio: 'inherit' })
  console.log('::endgroup::')

  if (res.error?.code === 'ENOENT') {
    fail('Go is required to run this action from source. Use the dist branch or run actions/setup-go first.')
  }
  if (res.error) fail(`go build: ${res.error.message}`)
  if (res.status !== 0) fail('go build failed')
  return out
}

function fail(msg) {
  console.log(`::error::${msg}`)
  process.exit(1)
}
