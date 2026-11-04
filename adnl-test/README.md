# ADNL test (branch adnl-test only)

Read-only TON Block-0 parser test. This directory lives ONLY on the
`adnl-test` branch of ton-meme-sniper-app and does NOT affect the
GitHub Pages app on `main` (index.html, style.css, app.js are untouched).

- Parser: main.go (Go, tonutils-go), no wallet, no key, no secret,
  no signing, no trade, no auto-buy, no Vault.
- No DEX address is hard-coded; without --dex-address,
  pool_candidate never fires.
- Workflow: .github/workflows/adnl-test.yml — runs on push to
  adnl-test and on workflow_dispatch. It builds, runs --once and
  --watch, then commits adnl-test/ADNL-RESULT.md back to this branch
  with a [skip ci] message, so the result is readable without a
  workflow-log API. The job step exit files (once.exit / watch.exit)
  are the truth; the job conclusion alone is NOT an ADNL pass.
