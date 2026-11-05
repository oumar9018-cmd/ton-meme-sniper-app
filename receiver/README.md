# Test receiver — TON Meme Sniper, Phase 1 only

Test receiver for the portable route. Python stdlib only, no ADNL,
no UDP, HTTPS only. Detection + alert-ready log only. No wallet,
no signing, no trade, no auto-buy, no Vault.

`pipeline.py` is the portable pipeline from
`trench-phase1/portable-route/`: webhook receiver (POST /webhook,
GET /health) -> fast pre-filter (one TonAPI account call) ->
enrich PASS candidates only (jetton + holders).

Run on Render (free, test only):

```bash
python3 receiver/pipeline.py --host 0.0.0.0 --port $PORT
```

Environment variables, set in the Render dashboard, never in code:

- `WEBHOOK_TOKEN` — Bearer check on POST /webhook. Required here.
- `TONAPI_KEY` — deliberately NOT set for this test. The receiver
  then runs in unauthenticated TonAPI mode (rate limited, about
  1 request per 4 seconds), so enrichment is slower and may hit
  429 under the firehose. That limit is documented, not hidden.

Render Free limits that apply to this test service: it spins down
after 15 minutes without inbound traffic and takes about a minute
to spin back up; its local JSONL logs are lost on spin-down /
restart; the in-memory duplicate guard resets on restart.
