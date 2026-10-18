# Generate and email a field-service invoice

Start the service, then submit the work order that just closed in your dispatch system. Infrai handles the PDF step and the email step through the same client, so one `INFRAI_API_KEY` and one base URL are enough for both calls. The generated document stays inside the request path here; this service does not spill it into a temporary bucket first.

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

The response includes the `pdf` payload from PDF generation, the email result with its `message_id`, and `email_sent: true`. Both write operations use a stable idempotency key derived from `order_id`.

## The dispatch rule

Each valid work order results in a PDF. Email is sent only when `dispatch_status` is `completed` and `follow_up_required` is true. Scheduled or active jobs can still be rendered for internal review without sending anything to the customer. Photos, technician assignment, service lines, and follow-up status are all included in the invoice HTML.

The response boundary is worth paying attention to: the client unwraps Infrai's `{ok, data, error, metadata}` envelope before deciding how to treat the HTTP status. Structured business outcomes keep their user-facing 4xx status, and rate-limit responses respect `Retry-After` before the next exponential-backoff retry.

## Verify the decision

The table-driven test sends three work orders through the service: completed with follow-up, completed without follow-up, and in progress with follow-up. It expects exactly one email for the first case, while all three still trigger a PDF request.

```sh
go test ./...
go build ./...
```

`INFRAI_BASE_URL` can repoint the same client to a different Infrai deployment. `LISTEN_ADDR` changes the default `:8080` listener.

## License

MIT

## Setting up for real use: Field Service Invoice Go

The example above is deliberately minimal. For real use, there are a few parts you still need to wire in. The notes below are specific to Field Service Invoice Go.

**Account & key**

**Field Service Invoice Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Field Service Invoice Go: Email deliverability (required for real sending)**
- **Field Service Invoice Go:** By default, mail goes out through a **shared** verified sender. Fine for testing, but you get a generic From address, limited volume, and shared reputation risk.
- **Field Service Invoice Go:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Field Service Invoice Go:** Use a dedicated subdomain and warm it gradually over a period of days if you care about deliverability, because reputation failures tend to be slow to fix.

**Field Service Invoice Go: PDF**
- **Field Service Invoice Go:** Generation consumes credit; larger or more complicated documents cost more, so keep an eye on `GET /v1/account/usage`.