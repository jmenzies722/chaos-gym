# Open questions

Comprehension questions from build sessions, captured automatically at the end
of each one. They are reviewed at the end of a phase.

Answer in your own words, then move the line to `## Answered` with the answer
underneath. Leaving one open is a legitimate outcome — it says which part of the
stack to go back to.

## Open
- [ ] 2026-08-12 — Which one, and why does being wrong make the others lie rather than just being one more broken panel?

- [ ] 2026-08-11 — The agent currently has one exporter, `debug`. When the gateway is added, spans need to reach Prometheus and still be visible locally. Given the Collector's `receivers → processors → exporters` shape, how many exporters can one pipeline have, and where should the `/healthz` filter go so it applies once rather than per-exporter?

## Answered

- [x] 2026-08-11 — A span showed `Parent` as all zeros and `SpanKind: 2`. If an instrumented caller hit `/work` over HTTP, which of the tracer provider's three pieces would make `Parent` stop being zeros, and what would have to appear on the wire?

  **Answer:** The propagator, and a `traceparent` HTTP header. Headers are the only
  one of method/path/body/headers that carries metadata without changing what the
  request means, so trace context stays invisible to anyone not participating.
  Verified: sending `traceparent: 00-0af765…-b7ad6b…-01` made the service adopt that
  trace ID and record `Parent.Remote: true`.

- [x] 2026-08-11 — The agent listens on the node's IP, not inside the app pod. Why can't the Go service's OTLP endpoint just be `localhost:4317`?

  **Josh:** container isolation, protocol schemes, port differences, or production
  topologies.

  **Sharpened:** right instinct, wrong boundary. `localhost` in Kubernetes is the
  *pod*, not the container — containers in one pod share a network namespace, so a
  sidecar Collector *would* be reachable on localhost. The agent is a DaemonSet, a
  separate pod with its own IP, so localhost hits the app's own loopback and the
  connection is refused silently on a background goroutine. Fixed with the downward
  API (`status.hostIP`).
