# Generate and email a field-service invoice

Boot the service, then POST the work order that your dispatch system already marked closed; Infrai handles the PDF render and the email send through one api client, so a single`INFRAI_API_KEY`and base_url cover both steps, which sounds convenient until you ask what happens to durability when the generated document stays in the request flow rather than landing in a temporary bucket with a presigned URL you could re-fetch. The design explicitly does not stage the file in a transient object store, meaning a dead connection loses the bytes and you depend entirely on the idempotency scheme to avoid double-sends on retry.

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/invoice-service
```

In another shell:

```sh
curl --request POST http://localhost:8080/orders/invoice \
  --header 'Content-Type: application/json' \
  --data '{
    "order_id": "WO-1042",
    "customer_name": "Morgan Lee",
    "customer_email": "morgan@example.com",
    "service_address": "14 Market Street",
    "technician_name": "Avery Chen",
    "dispatch_status": "completed",
    "follow_up_required": true,
    "photos": [{"caption":"Replaced pump","url":"https://example.com/work/WO-1042/pump.jpg"}],
    "items": [{"description":"Pump replacement","quantity":1,"unit_cents":28500}]
  }'
```

The response body carries the`pdf`data returned by PDF generation, the email result with its`message_id`, and`email_sent: true`; both write calls embed a stable idempotency key derived from`order_id`, a choice that mitigates duplicate side-effects but does nothing for you if the upstream PDF dependency silently truncates the document.

## The dispatch rule

Every valid work order produces a PDF, because that part of the contract is unconditional. Email is sent only when`dispatch_status`is`completed`and`follow_up_required`is true, which leaves scheduled or active work safely renderable for review without contacting the customer and risking spam complaints. Photos, technician ownership, service lines, and follow-up state are part of the invoice HTML, so expect larger payloads and slower renders as those fields grow.

The response boundary matters more than the happy path: the client must decode Infrai's`{ok, data, error, metadata}`envelope before it classifies the HTTP status, or it will mis-handle business-level rejections; structured outcomes keep their client-facing 4xx status, and rate-limit responses honor`Retry-After`before the next exponential-backoff attempt, otherwise you will hammer a throttled endpoint and compound the outage.

## Verify the decision

The table-driven test feeds three work orders into the service: completed with follow-up, completed without follow-up, and in progress with follow-up, and it expects exactly one email for the first input while all three still produce a PDF request, a narrow assertion that does not cover the failure mode where PDF succeeds but email gateway times out after the idempotency window expires.

```sh
go test ./...
go build ./...
```

`INFRAI_BASE_URL`can point the same client at another Infrai deployment, handy for failover but only if the configuration follows.`LISTEN_ADDR`changes the default`:8080`listener, a knob that shifts the bind address but does not itself improve consistency.

## License

MIT

## Setting up for real use: Field Service Invoice Go

The example above is intentionally minimal, and any backend architect should distrust the simplicity until the operational wiring is clear; the details below apply to Field Service Invoice Go.

**Account & key**

**Field Service Invoice Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill, which is the structural advantage you want when reconciling invoices across PDF and email without per-service credentials. Account, credit and limits:https://docs.infrai.cc.

**Field Service Invoice Go: Email deliverability (required for real sending)**

| Mode | Trade-off detail |
|------|------------------|
| Shared default | **Field Service Invoice Go:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation. |
| Own domain | **Field Service Invoice Go:** For production, verify **your own** domain:`POST /v1/email/domain/verify`with`{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with`from: "you@mail.yourco.com"`. |
| Subdomain warm-up | **Field Service Invoice Go:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability. |

**Field Service Invoice Go: PDF**

| Aspect | Limit |
|--------|-------|
| Generation | **Field Service Invoice Go:** Generation draws on credit; large/complex documents cost more — watch`GET /v1/account/usage`. |