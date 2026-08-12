"""Tests for the selection and safety logic.

These use real `kubernetes.client` model objects rather than mocks, so a field
rename in the client library breaks the test instead of silently passing. What
they do not do is talk to a cluster — the delete path is verified against the
real k3s cluster, because a fake API server would only prove the fake works.
"""

import random

import pytest
from kubernetes.client import V1ObjectMeta, V1Pod, V1PodStatus

from chaos_gym.scheduler import RefusedToKill, Target, choose_victim, run_once, running_pods


def pod(name: str, phase: str = "Running", terminating: bool = False) -> V1Pod:
    return V1Pod(
        metadata=V1ObjectMeta(
            name=name,
            deletion_timestamp="2026-08-11T00:00:00Z" if terminating else None,
        ),
        status=V1PodStatus(phase=phase),
    )


def test_running_pods_excludes_pending_and_terminating():
    pods = [pod("a"), pod("b", phase="Pending"), pod("c", terminating=True)]
    assert [p.metadata.name for p in running_pods(pods)] == ["a"]


def test_refuses_when_kill_would_breach_min_survivors():
    with pytest.raises(RefusedToKill):
        choose_victim([pod("only")], random.Random(0), min_survivors=1)


def test_kills_when_a_survivor_remains():
    victim = choose_victim([pod("a"), pod("b")], random.Random(0), min_survivors=1)
    assert victim.metadata.name in {"a", "b"}


def test_pending_pods_do_not_count_as_survivors():
    """Two pods, but only one is serving — killing it would leave zero."""
    with pytest.raises(RefusedToKill):
        choose_victim([pod("a"), pod("b", phase="Pending")], random.Random(0), min_survivors=1)


class FakeApi:
    """Records calls. Stands in for the API server, not for the client models."""

    def __init__(self, pods):
        self._pods = pods
        self.deleted = []

    def list_namespaced_pod(self, namespace, label_selector):
        self.last_query = (namespace, label_selector)
        return type("PodList", (), {"items": self._pods})()

    def delete_namespaced_pod(self, name, namespace):
        self.deleted.append((namespace, name))


def test_run_once_deletes_exactly_one_pod():
    api = FakeApi([pod("a"), pod("b")])
    target = Target(namespace="default", selector="app=chaos-app", min_survivors=1)
    name = run_once(api, target, random.Random(0), dry_run=False)
    assert api.deleted == [("default", name)]
    assert api.last_query == ("default", "app=chaos-app")


def test_dry_run_deletes_nothing():
    api = FakeApi([pod("a"), pod("b")])
    target = Target(namespace="default", selector="app=chaos-app", min_survivors=1)
    run_once(api, target, random.Random(0), dry_run=True)
    assert api.deleted == []
