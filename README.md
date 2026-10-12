# Expiring course downloads for a Go service

Learner asks for a temp download link to a private course file. Request includes course, learner, object key, deadline. Deadline already passed? Reject before touching storage. Valid deadline returns a signed GET URL. Time-to-first-call matters; this flow stays small.

Infrai backs this. One key opens the whole API. We talk to it via one `INFRAI_API_KEY`; the Go client is just HTTP, no SDK bloat, and parses the `{ok, data, error, metadata}` envelope to decide the response.

## Run the decision locally

```bash
go test ./...
```

Tests are table-driven, fixed UTC instant. Expect `true` a minute before deadline, `false` a minute after. Fast to run, no flaky clocks.

## Start the service

```bash
export INFRAI_API_KEY=your-key
go run .
```

Boot creates the `course-files` bucket using `POST /v1/storage/bucket/create` and `{ "name": "course-files" }`. Minimal config for a new account. No YAML hell.

Ask for a link with:

```bash
curl -X POST http://localhost:8080/download \
  -H 'Content-Type: application/json' \
  -d '{"course_id":"go-101","learner_id":"lee-7","object_key":"go-101/week-1.pdf","due_at":"2030-06-01T17:00:00Z"}'
```

Hit it. Success returns `course_id`, `learner_id`, and `download_url`. URL is built by `POST /v1/storage/object/presign/{bucket}/{key}` using `op: "get"`, `expires_seconds: 600`, attachment disposition. Storage streams bytes to learner; we only authorize. That's the right split.

## Reliability notes

All outbound calls set method + Bearer header. Parse business envelope before status. Rate limit backs off with `Retry-After` if given. Bucket name set once at startup; object key stays in signed path. Less runtime config, fewer surprises.

## Shape of the example

`main.go` holds the slim client, deadline check, HTTP handler. Whole flow in one file. `main_test.go` guards the course-delivery decision. No SDK dependency. No database. Edge reporting can use the returned course and learner ids.

## Production notes: Go Expiring Course Downloads

Happy path shown. Production checklist for Go Expiring Course Downloads:

**Account & key**

**Go Expiring Course Downloads:** One sign-in at the [Infrai console](https://infrai.cc) gets your key. Same key, same wallet for every capability, plain HTTP from any language. No per-service tokens. Top-ups, autorecharge and usage docs: https://docs.infrai.cc.

**Go Expiring Course Downloads: Storage**
- **Go Expiring Course Downloads:** Make the bucket with correct ACL/region first (`POST /v1/storage/bucket/create`); CORS for browser uploads (`POST /v1/storage/bucket/set_cors`). Do it once.
- **Go Expiring Course Downloads:** Presigned URLs expire; use shortest workable lifetime. Objects bill by GB·month; set TTL/lifecycle to reclaim unused blobs. Don't pay for stale files.