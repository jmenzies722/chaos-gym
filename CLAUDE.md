# Chaos Gym

Breaks a Kubernetes cluster on a schedule so Josh can practice diagnosing real
failures against real dashboards. Solo learning project — not shipped for other
users. Runs entirely on a local `kind` cluster by design, so the practice loop
costs nothing and nothing paid gets destroyed by a scheduler bug.

## Commands

```bash
# install
pip install -e .

# dev
kind create cluster --name chaos-gym

# test
pytest

# lint
ruff check .
```

Nothing beyond lint/test is wired up yet — this is a fresh repo. Don't add a
command here until it actually runs; a command that doesn't work is worse than no
command, because it gets run.

## Layout

- `src/chaos_gym/` — the chaos scheduler and anything using the Kubernetes Python
  client. Empty stub right now.
- `terraform/` — not created yet. Manages the `kind` cluster and its Helm releases
  (Prometheus/Grafana, Chaos Mesh) with **local state** — no remote backend, this
  is solo and free-tier by design.
- `k8s/` — not created yet. Raw manifests for the fake fleet of services that gets
  broken on purpose.

## Architecture

Nothing built yet. Phase 1 scope: a `kind` cluster running ~15-20 pods across a
handful of fake services, `kube-prometheus-stack` for dashboards, and one scheduled
job that kills a random pod. Done means Josh can look at Grafana and correctly name
what broke, without being told.

## Conventions

- `ruff` for lint and format, `pytest` for tests.
- Terraform with local state, not remote — this project never needs multi-person
  state locking.
- Kubernetes RBAC and NetworkPolicy stand in for IAM and security groups. That's a
  deliberate substitution, not a shortcut — see `DECISIONS.md` for why once it's
  written.

## Never

- Never provision or point anything at a paid cloud resource without discussing
  the cost and setting a hard budget alarm first. This project's whole design
  point is running at $0 on a local cluster — any move off that is a real decision,
  not a default.
- Never target a cluster other than the local `kind` cluster for chaos actions.
- Never edit `DECISIONS.md`. That file is Josh's, always, even when it would be
  faster for Claude to fill it in.
- Never generate a complete file for a piece Josh hasn't asked for yet — smallest
  piece that moves the current phase forward, then stop.
- Never commit secrets or real credentials to any file.
- Never run a destructive command (`terraform destroy`, deleting the cluster,
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
