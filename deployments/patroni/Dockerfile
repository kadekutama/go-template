# Citus 14.0 + Patroni v4.1.5 supervisor image (E07.1-T02).
# No public image bundles Citus and Patroni together, so the compose profile
# builds this image: the Citus Debian base plus the Patroni supervisor and a
# curl health probe. etcd provides the DCS; see ../docker/docker-compose.citus-patroni.yml.
FROM citusdata/citus:14.0

ENV DEBIAN_FRONTEND=noninteractive \
    PATRONI_VERSION=4.1.5

RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 python3-pip curl \
    && pip3 install --no-cache-dir --break-system-packages "patroni[etcd3]==${PATRONI_VERSION}" \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Patroni manages postgres; it must own the data directory at runtime.
RUN mkdir -p /var/lib/postgresql/data /etc/patroni \
    && chown -R postgres:postgres /var/lib/postgresql /etc/patroni

USER postgres

ENTRYPOINT ["patroni"]
CMD ["/etc/patroni/patroni.yml"]
