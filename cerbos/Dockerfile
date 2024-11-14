FROM ghcr.io/cerbos/cerbos:0.36.0

ENTRYPOINT ["/cerbos", "server", \
    "--config=/config.yaml", \
    "--set=server.httpListenAddr=:3593", \
    "--set=server.grpcListenAddr=:3593"]

EXPOSE 3593
