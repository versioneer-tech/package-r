FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    jq \
    git \
    mailcap \
 && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /db /workspace && chmod 777 /db /workspace

COPY healthcheck.sh /healthcheck.sh
COPY init.sh /init.sh
COPY filebrowser /filebrowser

RUN chmod 755 /filebrowser /init.sh /healthcheck.sh

RUN git clone https://github.com/versioneer-tech/package-r-design/ /package-r/img

ENV FB_BRANDING_NAME=packageR
ENV FB_BRANDING_FILES=/package-r
ENV FB_DATABASE=/db/bolt.db

HEALTHCHECK --start-period=2s --interval=5s --timeout=3s CMD /healthcheck.sh || exit 1

ENTRYPOINT ["bash", "-c", "./init.sh && ./filebrowser"]

EXPOSE 8080
