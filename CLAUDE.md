# Chaos Gym

Breaks a Kubernetes cluster on a schedule so Josh can practice diagnosing real
failures against real dashboards, while learning the stack Datadog-style
platform-engineering roles actually run: Go services, OpenTelemetry, and
distributed-systems failure modes. Solo learning project — not shipped for other
users. Runs on a single small EC2 instance running k3s (not managed EKS — its
~$73/mo control-plane fee buys HA nothing solo practice needs). Real AWS IAM,
VPC, and security groups; cheap, not free — see Never below for the cost guardrail
that has to exist before the instance does.

**2026-08-10 respec:** phase 1 target is now a real Go HTTP service (not generic
fake pods), instrumented with OpenTelemetry and shipping traces/metrics through an
OTel Collector into Prometheus/Grafana. The chaos scheduler that kills it stays
Python — both languages stay in the project on purpose. Postgres/Redis and the
rest of the failure-mode menu (latency injection, bad rollout, DB slowdown,
telemetry overload) are deferred to phase 2, so phase 1 doesn't try to teach five
new things at once.

## Commands

```bash
# install
pip install -e .

# dev
# terraform apply (from terraform/) once it exists — provisions the EC2 box and
# installs k3s via user-data.

# test
pytest

# lint
ruff check .
```

Nothing beyond lint/test is wired up yet — this is a fresh repo. Don't add a
command here until it actually runs; a command that doesn't work is worse than no
command, because it gets run.

## Layout

- `src/chaos_gym/` — the Python chaos scheduler (the thing doing the killing) and
  anything using the Kubernetes Python client. Empty stub right now.
- `app/` — the Go HTTP service that gets broken on purpose. `main.go`,
  `main_test.go`, and a two-stage `Dockerfile` (build on `golang`, run on
  `scratch`). OpenTelemetry instrumentation not added yet.
- `terraform/` — not created yet. First resource in here, before anything else:
  the AWS Budget alarm. Then the EC2 instance, its VPC/subnet/security group, and
  its IAM instance role. Local state — this project never needs multi-person state
  locking.
- `k8s/` — `app.yaml` (Deployment + Service for the Go app). Manifests for the
  OTel Collector (agent+gateway) and `kube-prometheus-stack` still to come.
  Applied to the k3s cluster on the EC2 box by copying to `/opt/chaos-gym/`
  over SSM, since the cluster has no inbound port open.

## Architecture

Built backwards from the done-state, then read forward as the build order: **Josh
can look at Grafana, correctly name what broke without being told, and explain how
a trace/metric got from the Go service to the dashboard.** Each step below only
exists because the step after it needs it.

1. AWS Budget alarm (Terraform) — nothing billable exists before this can watch
   it. **Done:** applied 2026-08-11, `chaos-gym-monthly`, $20/month, alerts to
   Josh's email at 80% actual / 100% forecasted.
2. VPC, subnet, security group (Terraform) — the network the instance lives in.
   **Done:** applied 2026-08-11. Public subnet in `us-east-1a`, IGW + route
   table for outbound internet, security group with zero inbound rules — SSM
   only, no SSH port open.
3. EC2 instance + IAM instance role (Terraform); k3s installed via user-data.
   **Done:** `i-02d3195a106ce38fd`, `t3.medium`, k3s v1.36.3+k3s1, node Ready.
   Reached over SSM with zero inbound ports open. Sizing was found by
   measurement, not guesswork: `t3.micro` ran `CPUCreditBalance` to 0 during
   the k3s install and throttled so hard SSM commands queued instead of
   executing; k3s idles at ~760MB, so `t3.small`'s 2 GiB would have left too
   little for `kube-prometheus-stack`. Budget Action also applied — auto-stops
   (never terminates) this one instance if *actual* spend hits 100%, scoped by
   instance ARN so the other project on this account is out of the blast
   radius. **Stop the instance between sessions:** `aws ec2 stop-instances
   --instance-ids i-02d3195a106ce38fd` (~$1.60/mo stopped, ~$35/mo running).
4. Go HTTP service — minimal, containerized, deployable. **Done:** `/healthz`
   and `/work` (returns hostname = pod name, which is what makes a pod kill
   visible from outside). Handles SIGTERM and drains via `srv.Shutdown`.
   Image is `scratch` + static binary, 2.5MB, cross-built `linux/amd64` from
   the arm64 Mac. Pushed to ECR, deployed to k3s as 2 replicas, load
   balancing verified 16/14 across 30 requests.
   **ECR pull secret expires every 12h** — re-run `/opt/chaos-gym/deploy.sh`
   on the instance at the start of a session to refresh it.
5. OpenTelemetry SDK instrumentation in the Go service — traces and metrics.
6. k8s manifests: deploy the Go service and the OTel Collector (agent) to k3s.
7. OTel Collector gateway config + `kube-prometheus-stack` (OTLP receiver +
   Grafana) deployed to k3s.
8. Python chaos scheduler — kills the Go service's pod on a schedule.
9. Verification: Josh reads Grafana, names the failure, and traces the telemetry
   path (Go service → Collector agent → Collector gateway → Prometheus → Grafana)
   unprompted. This is what "done" in the paragraph above actually means.

Phase 2 (not yet scoped in detail): Postgres/Redis behind the Go service, load
testing, and the rest of the failure-mode menu (latency injection, bad rollout,
DB slowdown, telemetry overload), ending in a written incident report.

## Conventions

- `ruff` for lint/format and `pytest` for tests on the Python side; `gofmt`/`go vet`
  and the standard `testing` package on the Go side.
- Terraform with local state, not remote — this project never needs multi-person
  state locking.
- k3s, not managed EKS. The instance's IAM role and security group are the real
  IAM/networking practice — no RBAC-as-substitute needed once this is real AWS.
- OpenTelemetry Collector, not a vendor-specific agent — the whole point of this
  phase is learning the vendor-neutral OTLP pipeline (instrumentation → Collector
  → backend), not a Datadog Agent shortcut.

## Never

- Never create the EC2 instance (or any billable resource) before the AWS Budget
  alarm exists. The alarm is phase 1, step 1 — everything billable comes after it,
  not before.
- Never provision anything beyond what phase 1 actually needs — one instance, one
  cheap instance type. No second instance, no NAT gateway, no load balancer,
  without discussing the cost first; those are the line items that turn "cheap"
  into a surprise bill.
- Never add Postgres/Redis or the wider failure-mode menu until phase 1 (Go
  service + OTel Collector + one failure mode) actually works end to end — that's
  phase 2 scope, not phase 1.
- Never edit `DECISIONS.md`. That file is Josh's, always, even when it would be
  faster for Claude to fill it in.
- Never generate a complete file for a piece Josh hasn't asked for yet — smallest
  piece that moves the current phase forward, then stop.
- Never commit secrets or real credentials to any file.
- Never run a destructive command (`terraform destroy`, terminating the instance,
  force-pushing) without asking first.

## How to work with me

- I'm learning this stack, not just shipping it. Explain the why before the how.
- Give me the smallest working piece, then stop. Don't generate whole files unless
  I ask.
- Name tradeoffs when there's a real choice to make.
- Ask me a comprehension question after each significant piece.
- Don't skip networking, IAM/RBAC, DNS, or state management. Those are the parts I
  need most.

## Known issues

None yet — repo was just provisioned, nothing built.

## References

See @README.md for setup and @DECISIONS.md for the record of what was decided and
why, in my own words.
