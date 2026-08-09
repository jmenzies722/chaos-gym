# Chaos Gym

Breaks a Kubernetes cluster on a schedule so Josh can practice diagnosing real
failures against real dashboards. Solo learning project — not shipped for other
users. Runs on a single small EC2 instance running k3s (not managed EKS — its
~$73/mo control-plane fee buys HA nothing solo practice needs). Real AWS IAM,
VPC, and security groups; cheap, not free — see Never below for the cost guardrail
that has to exist before the instance does.

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

- `src/chaos_gym/` — the chaos scheduler and anything using the Kubernetes Python
  client. Empty stub right now.
- `terraform/` — not created yet. First resource in here, before anything else:
  the AWS Budget alarm. Then the EC2 instance, its VPC/subnet/security group, and
  its IAM instance role. Local state — this project never needs multi-person state
  locking.
- `k8s/` — not created yet. Raw manifests for the fake fleet of services that gets
  broken on purpose, applied to the k3s cluster running on the EC2 box.

## Architecture

Nothing built yet. Phase 1 scope: one EC2 instance running k3s, ~15-20 pods across
a handful of fake services, `kube-prometheus-stack` for dashboards, and one
scheduled job that kills a random pod. Done means Josh can look at Grafana and
correctly name what broke, without being told.

## Conventions

- `ruff` for lint and format, `pytest` for tests.
- Terraform with local state, not remote — this project never needs multi-person
  state locking.
- k3s, not managed EKS. The instance's IAM role and security group are the real
  IAM/networking practice — no RBAC-as-substitute needed once this is real AWS.

## Never

- Never create the EC2 instance (or any billable resource) before the AWS Budget
  alarm exists. The alarm is phase 1, step 1 — everything billable comes after it,
  not before.
- Never provision anything beyond what phase 1 actually needs — one instance, one
  cheap instance type. No second instance, no NAT gateway, no load balancer,
  without discussing the cost first; those are the line items that turn "cheap"
  into a surprise bill.
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
