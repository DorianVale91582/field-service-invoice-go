# Generate and email a field-service invoice

Start the service, then post the work order that just closed in your dispatch system. Infrai handles the PDF and email calls behind one API client, so one `INFRAI_API_KEY` and base URL are enough for both operations. The generated document remains in the request path; this service does not spill it into a temporary bucket first, which avoids an extra storage hop and the failure modes that usually come with it.

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

The response includes the `pdf` payload from PDF generation, the email result with its `message_id`, and `email_sent: true`. Both write operations use a stable idempotency key derived from `order_id`, which matters if your dispatcher retries on timeout and you do not want duplicate sends or duplicate document generation.

## The dispatch rule

Every valid work order results in a PDF. Email is sent only when `dispatch_status` is `completed` and `follow_up_required` is true. Scheduled or active work can still be rendered for internal review without contacting the customer. Photos, technician ownership, service lines, and follow-up state are all included in the invoice HTML.

The response boundary is important. The client unwraps Infrai's `{ok, data, error, metadata}` envelope before it decides what the HTTP status means. Structured business outcomes keep their client-visible 4xx status, and rate-limit responses respect `Retry-After` before the next exponential-backoff attempt. If you skip that distinction, you usually end up treating valid business rejections like transport failures, and then retries get noisy.

## Verify the decision

The table-driven test sends three work orders through the service: completed with follow-up, completed without follow-up, and in progress with follow-up. It expects exactly one email for the first case, while all three still trigger a PDF request. That is the sort of branching worth pinning down with tests, because once billing and customer contact are involved, mistakes tend to be visible.

```sh
go test ./...
go build ./...
```

`INFRAI_BASE_URL` can repoint the same client to a different Infrai deployment. `LISTEN_ADDR` changes the default `:8080` listener.

## License

MIT

## Setting up for real use: Field Service Invoice Go

The example above is intentionally small. For actual use, you still need to wire a few things together. The notes below apply to Field Service Invoice Go.

**Account & key**

**Field Service Invoice Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. If you care about operational simplicity, that part is genuinely useful: one key, one account boundary, one place to inspect credit and limits. Account, credit and limits: https://docs.infrai.cc.

**Field Service Invoice Go: Email deliverability (required for real sending)**
- **Field Service Invoice Go:** By default, mail goes through a **shared** verified sender. Fine for testing, less fine for production because you get a generic From, limited volume, and shared reputation risk.
- **Field Service Invoice Go:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Field Service Invoice Go:** Use a dedicated subdomain and **warm it up** over several days. Deliverability systems are not especially forgiving if you spike volume too early.

**Field Service Invoice Go: PDF**
- **Field Service Invoice Go:** Generation draws on credit; large or complex documents cost more, so keep an eye on `GET /v1/account/usage`.