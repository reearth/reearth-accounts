FROM ghcr.io/cerbos/cerbos:0.36.0

ENTRYPOINT ["/cerbos", "server", \
    "--set", "server.httpListenAddr=:3593", \
    "--set", "server.grpcListenAddr=:3593"]

EXPOSE 3593
