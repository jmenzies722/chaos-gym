# syntax=docker/dockerfile:1

# Unlike the Go service, this cannot ship on scratch: the kubernetes client is
# Python, so the interpreter and its TLS certificate bundle have to come along.
# slim rather than full — the difference is roughly 900MB of build tooling that
# would only ever be attack surface at runtime.
FROM python:3.13-slim AS build
WORKDIR /src

# Dependency install is its own layer, keyed on pyproject.toml alone, so editing
# scheduler.py does not re-resolve and re-download the kubernetes client.
COPY pyproject.toml ./
RUN pip install --no-cache-dir --prefix=/install kubernetes>=31

COPY src/ ./src/
RUN pip install --no-cache-dir --prefix=/install --no-deps .

FROM python:3.13-slim
COPY --from=build /install /usr/local

# Non-root. The container needs no filesystem writes and no privileged
# operation — its only power comes from the ServiceAccount token Kubernetes
# mounts, which is scoped by RBAC rather than by Unix permissions.
RUN useradd --uid 65532 --create-home --shell /usr/sbin/nologin chaos
USER 65532:65532

ENTRYPOINT ["python", "-m", "chaos_gym.scheduler"]
