FROM debian:trixie-slim@sha256:8aa5a7bf5a1955bddbe7d74fcdd0b91ab30e74362a8b73af672b0481b35b44a0 AS accounts

RUN if ! getent group 100 >/dev/null; then groupadd --gid 100 package-r; fi \
 && useradd --uid 1000 --gid 100 --home-dir /home/package-r --no-create-home --shell /usr/sbin/nologin package-r \
 && mkdir -p /home/package-r \
 && chown -R 1000:100 /home/package-r

# Use Debian's libzstd1 fix until the runtime base includes it.
RUN apt-get update \
 && apt-get install --yes --no-install-recommends libzstd1 \
 && rm -rf /var/lib/apt/lists/* \
 && mkdir -p /runtime-inventory \
 && dpkg-query --status libzstd1 > /runtime-inventory/libzstd1 \
 && cp /var/lib/dpkg/info/libzstd1:amd64.md5sums /runtime-inventory/libzstd1.md5sums

FROM gcr.io/distroless/cc-debian13:nonroot@sha256:29ee70762214fe8190cea04ef1c0a84a8365879e047ce8f911ae5a5ab2355fa2

COPY --from=accounts /usr/lib/x86_64-linux-gnu/libzstd.so.1* /usr/lib/x86_64-linux-gnu/
COPY --from=accounts /usr/share/doc/libzstd1/ /usr/share/doc/libzstd1/
COPY --from=accounts /runtime-inventory/ /var/lib/dpkg/status.d/
COPY --from=accounts /etc/passwd /etc/group /etc/
COPY --from=accounts --chown=1000:100 /home/package-r /home/package-r
COPY --chmod=755 package-r /package-r

ENV HOME=/home/package-r
ENV PACKAGE_R_DATABASE=/tmp/package-r.db
ENV PACKAGE_R_ROOT=/
ENV PACKAGE_R_PORT=8888

HEALTHCHECK --start-period=2s --interval=5s --timeout=3s CMD ["/package-r", "healthcheck"]

USER 1000:100

ENTRYPOINT ["/package-r"]

EXPOSE 8888
