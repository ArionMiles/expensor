import fs from 'node:fs'

const [, , summaryPath, minPercentArg] = process.argv

if (!summaryPath || !minPercentArg) {
  console.error('usage: node check-frontend.mjs <coverage-summary.json> <min-percent>')
  process.exit(2)
}

if (!fs.existsSync(summaryPath)) {
  console.error(`frontend coverage summary not found: ${summaryPath}`)
  process.exit(1)
}

const summary = JSON.parse(fs.readFileSync(summaryPath, 'utf8'))
const actualPercent = summary.total?.statements?.pct
const minPercent = Number(minPercentArg)

if (typeof actualPercent !== 'number' || Number.isNaN(actualPercent)) {
  console.error(`failed to parse frontend statements coverage from ${summaryPath}`)
  process.exit(1)
}

console.log(`Frontend app coverage: ${actualPercent}% (minimum ${minPercent}%)`)

if (actualPercent < minPercent) {
  console.error('frontend app coverage is below the required floor')
  process.exit(1)
}
