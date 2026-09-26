FROM debian:bookworm-slim

ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
    busybox-static \
    ca-certificates \
    libstdc++6 \
 && rm -rf /var/lib/apt/lists/*

RUN if ! getent group 100 >/dev/null; then groupadd --gid 100 package-r; fi \
 && useradd --uid 1000 --gid 100 --home-dir /home/package-r --no-create-home --shell /usr/sbin/nologin package-r \
 && mkdir -p /home/package-r \
 && chown -R 1000:100 /home/package-r

COPY healthcheck.sh /healthcheck.sh
COPY init.sh /init.sh
COPY package-r /package-r

RUN chmod 755 /package-r /init.sh /healthcheck.sh

ENV HOME=/home/package-r
ENV PACKAGE_R_DATABASE=/tmp/package-r.db
ENV PACKAGE_R_ROOT=/
ENV PACKAGE_R_SERVER_PORT=8888

HEALTHCHECK --start-period=2s --interval=5s --timeout=3s CMD /healthcheck.sh || exit 1

USER 1000:100

ENTRYPOINT ["/init.sh", "--serve"]

EXPOSE 8888
