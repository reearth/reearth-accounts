FROM golang:1.24.2-alpine AS build
ARG TAG=release
ARG VERSION

RUN apk add --update --no-cache git ca-certificates build-base

WORKDIR /reearth-accounts-api

COPY go.mod go.sum main.go /reearth-accounts-api/
RUN go mod download

COPY cmd/ /reearth-accounts-api/cmd/
COPY pkg/ /reearth-accounts-api/pkg/
COPY internal/ /reearth-accounts-api/internal/

RUN CGO_ENABLED=0 go build -o reearth-accounts-api -tags "${TAG}" "-ldflags=-X main.version=${VERSION} -s -w -buildid=" -trimpath ./cmd/reearth-accounts

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /reearth-accounts-api/reearth-accounts-api /reearth-accounts-api/reearth-accounts-api

WORKDIR /reearth-accounts-api

CMD ["./reearth-accounts-api"]
