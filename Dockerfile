FROM alpine:latest

RUN apk --update add ca-certificates \
                     mailcap \
                     curl \
                     jq \
                     git \
                     bash \
                     build-base \
                     python3 \
                     py3-pip

RUN apk add git-annex

RUN curl -LO "https://dl.k8s.io/release/v1.31.2/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && \
    mv kubectl /usr/local/bin/

COPY healthcheck.sh /healthcheck.sh
RUN chmod +x /healthcheck.sh

COPY create-source.sh /usr/local/bin/create-source
RUN chmod +x /usr/local/bin/create-source

ENV PATH="/usr/local/bin:${PATH}"

HEALTHCHECK --start-period=2s --interval=5s --timeout=3s \
    CMD /healthcheck.sh || exit 1

VOLUME /srv
EXPOSE 80

RUN jq -n '{port: 80, baseURL: "", address: "", log: "stdout", database: "/database.db", root: "/srv"}' > /.package-r.json

COPY package-r /package-r
RUN chmod +x /package-r

ENTRYPOINT [ "/package-r" ]