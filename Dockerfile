FROM debian:trixie-slim AS accounts

RUN if ! getent group 100 >/dev/null; then groupadd --gid 100 package-r; fi \
 && useradd --uid 1000 --gid 100 --home-dir /home/package-r --no-create-home --shell /usr/sbin/nologin package-r \
 && mkdir -p /home/package-r \
 && chown -R 1000:100 /home/package-r

FROM gcr.io/distroless/cc-debian13:nonroot

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
