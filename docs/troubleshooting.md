# Troubleshooting

## Kubernetes Bucket Mount Health

If packageR runs on Kubernetes with a bucket-backed PVC, for example via a CSI
FUSE mount, packageR depends on the health of the mounted filesystem at
`FB_ROOT`. A stale mount can fail before packageR can browse, share, or presign
the affected files. A typical error is:

```text
Transport endpoint is not connected
```

You can verify this inside the pod:

```bash
stat /workspace
```

If the mount is broken, it returns an error similar to:

```text
stat: cannot statx '/workspace': Transport endpoint is not connected
```

Restarting the container may not fix a stale FUSE mount. In that case, recreate
the pod. The full csi-rclone example includes readiness and liveness probes
that check `/workspace` and keep packageR running as UID `1000` and GID `100`:
[`docs/examples/kubernetes-csi-rclone.yaml`](examples/kubernetes-csi-rclone.yaml).
