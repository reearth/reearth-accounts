FROM ghcr.io/cerbos/cerbos:0.36.0

COPY config.yaml /config.yaml

ENTRYPOINT ["/cerbos", "server", "--config=/config.yaml"]

EXPOSE 3593
