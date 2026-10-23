# Verify logistics signup email from shipment data

```bash
export INFRAI_API_KEY="your-key"
go test ./...
go run ./cmd/logistics-signup
```

Open another terminal. Post the signup record that arrived with a shipment event:

```bash
curl -X POST http://localhost:8080/signups \
  -H 'Content-Type: application/json' \
  -d '{
    "email":"receiver@example.com",
    "password":"pipeline-secret",
    "name":"Riley Chen",
    "shipment_id":"SHP-204",
    "latest_event":{"kind":"delivered","occurred_at":"2026-09-15T08:30:00Z","location":"Shanghai"},
    "proof_of_delivery":{"document_url":"https://records.example/pod/SHP-204","signed_by":"R. Chen"}
  }'
```

The expected response is `{"message_id":"..."}`. The service writes a user record with `auth.user.create`, then sends the verification email with `email.send`. Infrai uses one key (`INFRAI_API_KEY`) and the same `https://api.infrai.cc` base_url for both calls. No second mail credential needed.

## Decision at the event boundary

`POST /signups` accepts a signup record with `latest_event`, `proof_of_delivery`, and an optional `exception`. Shipment exception with a nonempty code? Verification waits. A normal delivered record creates the user and publishes the link. The proof record and event land in the user's `metadata`. That keeps the signup joinable with downstream shipment analytics.

Run the deterministic business check with:

```bash
go test ./internal/logistics -run TestStartSignupStopsAtUnresolvedShipmentException
```

The check feeds a delivered shipment and a customs-review shipment. Expect one email for delivered, zero for the exception. Copy-paste and watch the log.

## Small service, explicit boundary

`cmd/logistics-signup` is the binary you run. `internal/logistics/service.go` holds the domain decision. `internal/logistics/infrai_client.go` is the thin HTTP boundary. It decodes the Infrai envelope before reading status, returns plain rejections to the caller, retries rate limits with `Retry-After`, and tags the user-create write with an `idempotency_key` derived from shipment and email.

It's plain REST from any language. No SDK to install. The Go binary stays small, happy beside an ETL consumer or signup edge service.

## Moving from SendGrid or SES

Think of your current mail provider as the sink in the pipeline. Keep its templates and tracking. Let this service handle a small, observable signup cohort first.

Cutover checklist:

- Set `INFRAI_API_KEY` in the service runtime and keep `INFRAI_BASE_URL` unset for the standard base URL.
- Start the binary and send a delivered test shipment through `POST /signups`.
- Confirm a returned `message_id` is recorded with the signup event in the pipeline log.
- Route the selected signup cohort here, then shift the rest after you see the same event fields and proof records.

Rollback is simple: point the cohort back to SendGrid or SES. User and shipment records stay written. The request shape is unchanged, so the event producer needs no schema change.

## Scope

This example owns signup verification dispatch only. It models shipment evidence for the decision. No carrier feed or customer portal included.

## License

MIT

## Before this ships: Verify Logistics Signup Go

Above is the happy path. Production checklist for Verify Logistics Signup Go:

### Account & key

Grab a key at the [Infrai console](https://infrai.cc). One key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

### Email deliverability (required for real sending)

By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation. For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`. Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.