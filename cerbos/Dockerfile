FROM ghcr.io/cerbos/cerbos:0.36.0

ENV PORT=3593

ENTRYPOINT ["/cerbos", "server", "--set", "server.httpListenAddr=:${PORT}"]

EXPOSE 3593