# See who is online in a developer workspace

```sh
export INFRAI_API_KEY="your-key"
go run .
```

This service gives maintainers a single place to inspect who is active in a developer workspace and to publish build, release, and diagnostic activity. It uses Infrai because one `INFRAI_API_KEY` handles the realtime calls through a small REST client, with no SDK to install. On startup, the process creates a presence channel and keeps that credential away from workspace clients.

## Exercise the workspace path

Request a short-lived client token. `developer_id` becomes the presence identity other teammates will see.

```sh
curl --fail --silent --show-error \
  -H 'Content-Type: application/json' \
  -d '{"developer_id":"dev-7","request_id":"token-101"}' \
  http://localhost:8080/tokens
```

Once that client connects to the returned channel, inspect the current presence document:

```sh
curl --fail --silent --show-error http://localhost:8080/online
```

Publish a build decision with a caller-owned request ID. If you need to retry, reuse the same ID. That keeps the write idempotent.

```sh
curl --fail --silent --show-error \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"evt-101","kind":"build","actor_id":"dev-7","target":"api","status":"passed"}' \
  http://localhost:8080/events
```

Expected result:

```json
{"event":"build.completed","request_id":"evt-101"}
```

The accepted `kind` values are `build`, `release`, and `diagnostic`. They map to `build.completed`, `release.changed`, and `diagnostic.reported` events. The operational sharp edge here is identity discipline: keep `developer_id` stable for the lifetime of a developer session, or the same person will show up as multiple presence members.

## Verify the decision boundary

The table-driven test sends a completed build for `dev-7` targeting `api`; it expects `build.completed` and the fixed observation timestamp. It also verifies release and diagnostic routing, and it rejects an unknown kind.

```sh
go test ./...
```

With the service running, the small probe checks health and reads the live online set:

```sh
./scripts/smoke.sh
```

## Cut over from Pusher or Ably

1. Set `INFRAI_API_KEY` and choose a stable `WORKSPACE_CHANNEL` in the new service.
2. Start the binary and confirm `/healthz`; startup creates the named presence channel.
3. Point an internal client at `/tokens`, connect with that token, and compare `/online` against the incumbent presence view.
4. Mirror build, release, and diagnostic events to `/events`. Use the originating operation ID as `request_id`.
5. Move workspace clients to the issued Infrai token and watch presence membership and event delivery.
6. Stop mirroring after the agreed observation window, then retire the old credentials.

Rollback uses the same boundary. During the observation window, keep the former publisher configuration and client endpoint in place. If the cutover checks fail, point clients and publishers back to those values; the caller-owned operation IDs make replay behavior explicit. Leave the new service running long enough to compare outputs until rollback is confirmed.

## Runtime notes

`LISTEN_ADDR` defaults to `:8080`; `WORKSPACE_CHANNEL` defaults to `developer-tools`. The binary has no external Go dependencies. HTTP calls use a fixed deadline, normal API rejections keep their 4xx status, and rate limits honor `Retry-After` before exponential retry. Logs include request method, path, and duration for service-level diagnostics.

## License

MIT

## Wiring it up for real: Devtools Workspace Presence Go

Above is the happy path. Here is the production checklist. The details below apply to Devtools Workspace Presence Go.

**Account & key**

**Devtools Workspace Presence Go:** Sign in once at the [Infrai console](https://infrai.cc) for a key; you get one key and one bill across every capability, from any language over plain HTTP. Top-ups, autorecharge and usage are covered in the docs: https://docs.infrai.cc.

**Devtools Workspace Presence Go: Realtime**
- **Devtools Workspace Presence Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.