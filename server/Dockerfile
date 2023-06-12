FROM golang:1.20-alpine AS build
ARG TAG=release
ARG REV
ARG VERSION

RUN apk add --update --no-cache git ca-certificates build-base
COPY go.mod go.sum main.go /app/
WORKDIR /app
RUN go mod download
COPY cmd/ /app/cmd/
COPY internal/ /app/internal/
RUN CGO_ENABLED=0 go build -tags "${TAG}" "-ldflags=-X main.version=${VERSION} -s -w -buildid=" -trimpath ./cmd/reearth-account

FROM gcr.io/distroless/static
ENTRYPOINT ["/reearth-account"]
COPY --from=build /app/reearth-account /
