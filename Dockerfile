FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    bash \
    unzip \
    curl \
    vim \
    git \
    jq \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

ENV KUBECTL_VERSION=1.31.6
RUN curl -LO "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/

ENV BOLTBROWSER_VERSION=2.2
ENV BOLTBROWSER_URL=https://github.com/br0xen/boltbrowser/releases/download/${BOLTBROWSER_VERSION}/boltbrowser.linux64
RUN curl -L $BOLTBROWSER_URL -o /usr/local/bin/boltbrowser && \
    chmod +x /usr/local/bin/boltbrowser

RUN git clone https://github.com/versioneer-tech/package-r-design/ /package-r/img

COPY cli/ /usr/local/bin/
RUN chmod +x /usr/local/bin/*

COPY healthcheck.sh /healthcheck.sh
RUN chmod +x /healthcheck.sh

HEALTHCHECK --start-period=2s --interval=5s --timeout=3s \
    CMD /healthcheck.sh || exit 1

ENV FB_DATABASE=/db/bolt.db

COPY filebrowser /filebrowser
RUN chmod +x /filebrowser

COPY init.sh /init.sh
RUN chmod +x /init.sh

ENTRYPOINT ["bash","-c","./init.sh && ./filebrowser"]

EXPOSE 8080