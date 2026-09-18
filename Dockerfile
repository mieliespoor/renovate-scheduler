# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY *.go ./

ARG VERSION=dev

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /out/renovate-scheduler .

FROM gcr.io/distroless/static:nonroot

WORKDIR /app
COPY --from=build /out/renovate-scheduler /usr/local/bin/renovate-scheduler

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/renovate-scheduler"]
