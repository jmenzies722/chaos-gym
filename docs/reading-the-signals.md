# Reading the signals

How to interpret the dashboard, and where to go when it tells you something is
wrong. This is the reference for the actual exercise: look, decide what
happened, *then* check.

---

## The three altitudes

Diagnosis is reading down a stack, and the dashboard is laid out in the order
you should read it.

**Service layer** — request rate, latency, errors. The user's view: is anyone
being hurt? It's the only layer that ultimately matters, and it can never tell
you *why*.

**Platform layer** — pod lifecycle, running replicas. The cause. On its own a
pod dying means nothing; it only becomes information when paired with whether
the service layer flinched.

**Pipeline layer** — spans in vs out, refused/failed, data age. Whether to
believe the other two. Without it, a dead Collector and a healthy quiet service
draw identical graphs.

**Read data age first.** If the pipeline is stale, everything above it is a
photograph of the past and no conclusion drawn from it is safe.

---

## What normal looks like

Know these cold. You will spot an anomaly because these numbers are in your
head, not because a panel turns red.

| Signal | Healthy value | Why that number |
|---|---|---|
| Running replicas | `2` | `replicas: 2` in the Deployment |
| Request rate | `~0.9 /s` | The load generator's 1/s, minus its own sleep and round trip |
| p50 latency | `100ms` | `WORK_DURATION=100ms` |
| p95 latency | `~109ms` | Base plus scheduling jitter |
| Non-2xx | `0` | Flat zero line, **not** "No data" |
| Data age | sawtooth `0–15s` | The spanmetrics flush interval; it resets each batch |
| Spans accepted | `~3 /s` | Summed across both Collectors, so roughly double the app's rate |
| Refused / failed | `0` | Any non-zero value means telemetry is being dropped |

Two of those deserve emphasis. **Non-2xx must read zero, never "No data"** — an
empty PromQL result and a genuinely healthy service look identical otherwise,
which is why the query ends in `or vector(0)`. And **data age sawtooths** — a
value that climbs steadily without resetting means the pipeline stopped, and
every graph above it froze rather than went quiet.

---

## Failure signatures

Each of these looks different, and telling them apart from the dashboard alone
is the whole skill.

### A pod was killed (the scheduled chaos)

| Panel | What happens |
|---|---|
| Pod lifecycle | One line ends, a new one begins seconds later |
| Running replicas | Dips `2 → 1 → 2`, **or shows nothing at all** |
| Request rate | Unchanged |
| p95 | Small bump — requests caught mid-drain |
| Non-2xx | Stays at zero |

The replicas panel often misses the dip entirely: `kube-state-metrics` samples
every 30 seconds and the replacement pod is Running within about a second, so
the `2 → 1` moment can fall between two samples. That is sampling resolution,
not a broken panel, and it's exactly why the lifecycle panel exists alongside it.

**Errors staying at zero means the SIGTERM drain worked.** But note what that
claim covers: these metrics come from *server-side* spans, so they only count
requests the service accepted. A request refused at the connection level
produces no span at all and shows up as a **dip in request rate**, not as an
error. "Errors flat" means nothing accepted was lost — it is not by itself proof
that no client saw a failure.

### A replica is degraded (injected latency)

| Panel | What happens |
|---|---|
| p50 | **Unchanged** |
| p95 / p99 | Jump to a plateau at the injected delay |
| Non-2xx | Zero |
| Replicas / restarts | Unchanged |
| Request rate | Sags slightly — a sequential client blocked longer issues fewer requests |

Nothing in Kubernetes notices this. The liveness probe hits `/healthz`, which
the fault does not touch, so no probe fails and nothing restarts. This is the
gap between "the platform says healthy" and "users are suffering," and only a
percentile catches it. An average would read around 380ms — vaguely elevated,
explaining nothing.

### The telemetry pipeline broke

| Panel | What happens |
|---|---|
| Request rate | Falls |
| Spans accepted / sent | Falls **at the same instant** |
| Data age | Climbs without resetting |
| Pod lifecycle | Unchanged — the app pods are fine |

The tell is the correlation. If request rate falls *and* the pipeline panels
fall together, the requests were served and their spans were lost. If request
rate falls and the pipeline is healthy, traffic genuinely stopped. Two very
different investigations, and only the pipeline row distinguishes them.

### Something was OOMKilled

Not visible on this dashboard at all — which is itself worth knowing. Go
looking at pod restarts and exit codes. **Exit 137 is `128 + 9`: SIGKILL**, the
kernel enforcing a cgroup memory limit with no grace period. The signature in
the logs is a container log that ends mid-normal-operation with no error,
because a SIGKILLed process gets no chance to write one.

> A log ending with an error means the process crashed.
> A log that just stops means something killed it.

---

## Where the logs are

Everything runs over SSM; there is no inbound port and no kubectl from a laptop.
Set this first:

```bash
export INSTANCE_ID=$(terraform -chdir=terraform output -raw instance_id)

# helper: run anything on the cluster
k() {
  local id
  id=$(aws ssm send-command --instance-ids "$INSTANCE_ID" \
        --document-name AWS-RunShellScript \
        --parameters "commands=[\"$*\"]" --query Command.CommandId --output text)
  sleep 5
  aws ssm get-command-invocation --command-id "$id" \
    --instance-id "$INSTANCE_ID" --query StandardOutputContent --output text
}
```

Then:

| What you want | Command |
|---|---|
| The app's own logs | `k "k3s kubectl logs -l app=chaos-app --tail=50 --prefix"` |
| Whether the drain worked | `k "k3s kubectl logs -l app=chaos-app --tail=50 \| grep -i drain"` |
| What the scheduler killed | `k "k3s kubectl -n chaos logs -l job-name --tail=20"` |
| Recent chaos runs | `k "k3s kubectl -n chaos get jobs"` |
| Collector agent | `k "k3s kubectl -n observability logs daemonset/otel-agent --tail=50"` |
| Collector gateway | `k "k3s kubectl -n observability logs deployment/otel-gateway --tail=50"` |
| Grafana | `k "k3s kubectl -n observability logs deploy/kube-prometheus-stack-grafana -c grafana --tail=50"` |
| Why a pod restarted | `k "k3s kubectl describe pod <name> \| grep -A4 'Last State'"` |
| Cluster events (the fastest first look) | `k "k3s kubectl get events -A --sort-by=.lastTimestamp \| tail -20"` |
| **Logs from the container that died** | `k "k3s kubectl logs <pod> --previous"` |

That last one is the important one and the easiest to forget. `logs <pod>` gives
you the *current* container; after a restart the interesting output is in the
one that died, and only `--previous` shows it.

The Collectors log at `verbosity: basic`, so they report a count per batch
rather than every span. To see individual spans again while debugging, set
`verbosity: detailed` in `k8s/otel-agent.yaml`, redeploy, and remember to put it
back — at detailed the same span is written to disk twice, once per Collector.

## From a dashboard anomaly to the right log

1. **Data age climbing?** Collector logs first. Nothing above it is trustworthy.
2. **Request rate fell?** Check whether the pipeline panels fell with it. Together
   means telemetry; alone means traffic.
3. **p95 up, p50 flat?** One replica is degraded. Compare pods — the response
   body returns the pod name, so `curl` a few times and see which one is slow.
4. **Replicas below 2, or restarts climbing?** `describe pod`, look for
   `Last State` and the exit code, then `logs --previous`.
5. **Non-2xx above zero?** The drain failed. App logs, grep for `drain`, and
   check whether `terminationGracePeriodSeconds` still exceeds the server's
   `shutdownGrace`.
