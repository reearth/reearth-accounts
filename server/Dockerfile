FROM golang:1.20-alpine AS build
ARG TAG=release
ARG VERSION

RUN apk add --update --no-cache git ca-certificates build-base

WORKDIR /reearth-dashboard

COPY go.mod go.sum main.go /reearth-dashboard/
RUN go mod download

COPY cmd/ /reearth-dashboard/cmd/
COPY pkg/ /reearth-dashboard/pkg/
COPY internal/ /reearth-dashboard/internal/

RUN CGO_ENABLED=0 go build -tags "${TAG}" "-ldflags=-X main.version=${VERSION} -s -w -buildid=" -trimpath ./cmd/reearth-dashboard

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /reearth-dashboard/reearth-dashboard /reearth-dashboard/reearth-dashboard

WORKDIR /reearth-dashboard

CMD ["./reearth-dashboard"]
