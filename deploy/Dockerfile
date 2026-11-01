# pass-api for JODA (linux/amd64). Pure Go, static binary on distroless.
# Build from the repo root: docker build -f deploy/Dockerfile -t ghcr.io/lioilsources/pass-api:latest .
FROM golang:1.26-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/pass-api ./cmd/pass-api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/pass-api /usr/local/bin/pass-api
ENV PASS_ADDR=:8096 PASS_DB_PATH=/data/pass.db
EXPOSE 8096
VOLUME /data
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD ["/usr/local/bin/pass-api", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/pass-api"]
CMD ["serve"]
