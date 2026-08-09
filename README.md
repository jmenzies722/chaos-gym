# Chaos Gym

Breaks a Kubernetes cluster on a schedule so I can practice diagnosing real
failures against real dashboards. Runs on a single small EC2 instance (k3s, not
managed EKS) — real AWS IAM/VPC, kept cheap on purpose, with an AWS Budget alarm
as the first thing built, before any billable resource exists.

Status: just provisioned, phase 1 not started. See `DECISIONS.md` for the record
of what's been decided and why, and `CLAUDE.md` for how this repo works.

Architecture diagram and demo link go here once phase 1 ships.
