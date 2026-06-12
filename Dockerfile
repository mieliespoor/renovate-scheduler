# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/renovate-scheduler .

FROM gcr.io/distroless/static:nonroot

WORKDIR /app
COPY --from=build /out/renovate-scheduler /usr/local/bin/renovate-scheduler
COPY config.toml renovate-repos.json ./

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/renovate-scheduler"]
