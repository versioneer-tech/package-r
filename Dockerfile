FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive PYTHONUNBUFFERED=1 PIP_NO_CACHE_DIR=1

RUN apt-get update && apt-get install -y \
    bash \
    unzip \
    curl \
    vim \
    git \
    jq \
    fuse \
    libfuse-dev \
    python3 \
    python3-pip \
    s3fs \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN python3 -m pip install --upgrade pip

ENV KUBECTL_VERSION=v1.31.2
ENV KUBECTL_URL=https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl
RUN curl -L $KUBECTL_URL -o /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl 

ENV AWS_CLI_VERSION=2.7.0
ENV AWS_CLI_URL=https://awscli.amazonaws.com/awscli-exe-linux-x86_64-${AWS_CLI_VERSION}.zip
RUN curl $AWS_CLI_URL -o awscliv2.zip \
    && unzip awscliv2.zip \
    && ./aws/install --bin-dir /usr/local/bin \
    && aws --version \
    && rm -rf awscliv2.zip aws

ENV BOLTBROWSER_VERSION=2.2
ENV BOLTBROWSER_URL=https://github.com/br0xen/boltbrowser/releases/download/${BOLTBROWSER_VERSION}/boltbrowser.linux64
RUN curl -L $BOLTBROWSER_URL -o /usr/local/bin/boltbrowser && \
    chmod +x /usr/local/bin/boltbrowser

RUN git clone https://github.com/versioneer-tech/package-r-design/

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