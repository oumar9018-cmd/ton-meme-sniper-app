# ADNL test result (GitHub Actions, branch adnl-test)

Read-only block parsing only. No wallet, no key, no signing, no trade.
Job conclusion alone is NOT an ADNL pass; the exit files below are the truth.

## Run

- run_id: $GITHUB_RUN_ID (see workflow run for this id)
- run_number: $GITHUB_RUN_NUMBER
- ref: $GITHUB_REF
- sha: $GITHUB_SHA

## Build

```
go: downloading github.com/xssnick/tonutils-go v1.19.1
go: downloading golang.org/x/net v0.59.0
go: downloading golang.org/x/sys v0.48.0
go: downloading filippo.io/edwards25519 v1.2.0
go: downloading github.com/pierrec/lz4/v4 v4.1.28
vet_exit=0
build_exit=0
-rwxr-xr-x 1 runner runner 11723609 Nov  4 13:05 trench-block0-parser

```

## --once

- exit: ?
- start_epoch: 1793797544, end_epoch: ?
- json_events: 3, deploy: 0, pool_candidate: 0
- master_seqnos_in_events: [103324637]
- latency: latency not computable: invalid literal for int() with base 10: '?'

## --watch (max 5 master blocks)

- exit: ?
- json_events: 3, deploy: 0, pool_candidate: 0
- master_seqnos_in_events: [103324640]
- max_block_time: 1793797545

## ERROR / stage lines (first 20)

```
(none)
```

## INFO lines (first 20)

```
INFO dex_address not supplied: pool_candidate will never be emitted; deploys are logged as 'deploy'. TODO: supply a verified DEX factory/router address via --dex-address.
INFO master workchain=-1 shard=-9223372036854775808 seqno=103324637
INFO dex_address not supplied: pool_candidate will never be emitted; deploys are logged as 'deploy'. TODO: supply a verified DEX factory/router address via --dex-address.
INFO master workchain=-1 shard=-9223372036854775808 seqno=103324640
```

## Pass criteria (draft)

1. ADNL connect succeeds (no stage=connect error): FAIL
2. --once real JSON events incl. a deploy: FAIL
3. --watch parses successive masters without hang/dup: FAIL / NOT PROVEN
4. Measured GenUtime vs first-seen latency: run-level upper bound only, see latency line above; no <2s claim unless per-event proof exists.

