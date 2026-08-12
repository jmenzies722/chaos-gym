# syntax=docker/dockerfile:1

FROM golang:1.26 AS build
WORKDIR /src

# Copy the module files first so dependency download is cached separately from
# source changes — editing main.go should not re-download the module graph.
COPY go.mod ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a static binary with no libc dependency, which is what
# makes the scratch-based runtime stage below possible. TARGETARCH is set by
# buildx from the --platform flag.
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /out/app .

# scratch has no shell, no package manager, and no CA certificates — nothing an
# attacker can pivot to if they get execution. The tradeoff is that `kubectl
# exec` into this container gets you nothing, so debugging happens through
# telemetry rather than by poking around inside.
FROM scratch
COPY --from=build /out/app /app
EXPOSE 8080
USER 65534:65534
ENTRYPOINT ["/app"]
