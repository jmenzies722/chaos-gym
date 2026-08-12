"""Kill one pod of the target Deployment, then exit.

WHY THIS DOES NOT LOOP
Scheduling is Kubernetes' job. This runs as a CronJob: one execution, one kill,
exit. A long-running Python process that sleeps between kills is a second thing
that has to stay alive and be monitored, and when it dies quietly the chaos
simply stops without anyone noticing. A CronJob gives retries, run history, and
a declarative schedule for free. The cost is one-minute granularity, which is
finer than anything this project needs.

Safety is structural rather than careful. The namespace and label selector are
required, and the run aborts if killing would leave fewer than MIN_SURVIVORS
pods — a chaos tool that can take the whole service down is an outage
generator, not a teaching aid.
"""

from __future__ import annotations

import logging
import os
import random
import sys
from dataclasses import dataclass

from kubernetes import client, config
from kubernetes.client.rest import ApiException

log = logging.getLogger("chaos_gym")


@dataclass(frozen=True)
class Target:
    namespace: str
    selector: str
    min_survivors: int


class RefusedToKill(Exception):
    """Raised when killing would breach a safety rule. Not an error condition."""


def running_pods(pods: list) -> list:
    """Only pods actually serving traffic are candidates.

    A pod already Pending or Terminating is not carrying requests, so killing it
    proves nothing and wastes the run — the dashboard would show no change and
    the exercise would look broken rather than uneventful.
    """
    return [
        p for p in pods if p.status.phase == "Running" and p.metadata.deletion_timestamp is None
    ]


def choose_victim(pods: list, rng: random.Random, min_survivors: int):
    """Pick one running pod at random, or refuse.

    Random rather than oldest-first on purpose: a predictable victim lets you
    learn the pattern instead of the diagnosis.
    """
    candidates = running_pods(pods)
    if len(candidates) - 1 < min_survivors:
        raise RefusedToKill(
            f"{len(candidates)} running pod(s); killing one would leave fewer than "
            f"{min_survivors}. Nothing killed."
        )
    return rng.choice(candidates)


def run_once(api: client.CoreV1Api, target: Target, rng: random.Random, dry_run: bool) -> str:
    pods = api.list_namespaced_pod(target.namespace, label_selector=target.selector).items
    victim = choose_victim(pods, rng, target.min_survivors)
    name = victim.metadata.name

    if dry_run:
        log.info("DRY RUN: would delete pod %s/%s", target.namespace, name)
        return name

    # Deleting the pod, not the Deployment. The ReplicaSet notices the shortfall
    # and creates a replacement — which is the behaviour under test. The pod
    # gets SIGTERM and its own terminationGracePeriodSeconds to drain.
    api.delete_namespaced_pod(name=name, namespace=target.namespace)
    log.info("deleted pod %s/%s", target.namespace, name)
    return name


def target_from_env() -> Target:
    namespace = os.environ.get("CHAOS_NAMESPACE", "").strip()
    selector = os.environ.get("CHAOS_SELECTOR", "").strip()
    if not namespace or not selector:
        raise SystemExit("CHAOS_NAMESPACE and CHAOS_SELECTOR are required — refusing to guess.")
    return Target(
        namespace=namespace,
        selector=selector,
        min_survivors=int(os.environ.get("CHAOS_MIN_SURVIVORS", "1")),
    )


def main() -> int:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    target = target_from_env()
    dry_run = os.environ.get("CHAOS_DRY_RUN", "").lower() in {"1", "true", "yes"}

    # In-cluster config reads the ServiceAccount token the kubelet mounts into
    # the pod. There is no kubeconfig here and no credential in the image.
    config.load_incluster_config()

    try:
        run_once(client.CoreV1Api(), target, random.SystemRandom(), dry_run)
    except RefusedToKill as exc:
        # Exit 0: the safety rule working as designed is not a failed run, and
        # a non-zero exit would make the CronJob retry and fill the log with
        # false alarms.
        log.warning("%s", exc)
        return 0
    except ApiException as exc:
        log.error("kubernetes API refused the call: %s %s", exc.status, exc.reason)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
