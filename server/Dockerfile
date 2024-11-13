FROM golang:1.20-alpine AS build
ARG TAG=release
ARG VERSION

RUN apk add --update --no-cache git ca-certificates build-base

WORKDIR /reearth-dashboard-api

COPY go.mod go.sum main.go /reearth-dashboard-api/
RUN go mod download

COPY cmd/ /reearth-dashboard-api/cmd/
COPY pkg/ /reearth-dashboard-api/pkg/
COPY internal/ /reearth-dashboard-api/internal/

RUN CGO_ENABLED=0 go build -o reearth-dashboard-api -tags "${TAG}" "-ldflags=-X main.version=${VERSION} -s -w -buildid=" -trimpath ./cmd/reearth-dashboard

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /reearth-dashboard-api/reearth-dashboard-api /reearth-dashboard-api/reearth-dashboard-api

WORKDIR /reearth-dashboard-api

CMD ["./reearth-dashboard-api"]
