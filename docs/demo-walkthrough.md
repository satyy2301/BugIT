# Demo Walkthrough: Checkout Payment 500

## What is the bug?

A customer tries to checkout order **ORD-8842**. The flow fails like this:

1. **api-gateway** forwards `POST /checkout` to **order-api**
2. **order-api** calls **payment-service** with `POST /charge`
3. **payment-service** returns **HTTP 500** — card processor is down
4. **order-api** returns **HTTP 502** to the client

**Root cause:** `payment-service` upstream error `card_processor_down`.

## Load the demo snapshot

### VS Code extension

1. Open `extensions/vscode` and press **F5**
2. In the Extension Development Host: **Ctrl+Shift+P** → **DRE: Load Snapshot**
3. Pick `test/fixtures/demo-checkout-500.dre`

You should see:

- Red **bug banner** with title and root cause
- **Event timeline** with HTTP steps (500 row highlighted in red)
- **Cross-service flow** string
- **Replay proxy** address (`127.0.0.1:18080`)

### Command line

```powershell
.\bin\dre-replay.exe load --format ide --dre test\fixtures\demo-checkout-500.dre
.\bin\dre-replay.exe run --dre test\fixtures\demo-checkout-500.dre
```

## What does "replay" mean?

Replay starts a **local fake network** on your machine. If you run your service locally and point it at `127.0.0.1:18080`, it receives the **same responses** that were recorded — including the 500 error — without calling real production APIs.

## Regenerate the fixture

```powershell
go run ./scripts/generate-demo-snapshot -out test/fixtures/demo-checkout-500.dre
```

## Live mock capture

Set `DRE_MOCK_SCENARIO=checkout_500` on `dre-agent` to emit the same scenario into a live collector.
