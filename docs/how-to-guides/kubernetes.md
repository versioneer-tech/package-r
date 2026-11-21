# Run packageR on Kubernetes

The runtime image has no shell. Prepare the database with direct packageR
commands in separate init containers. They share an `emptyDir` volume:

1. The init containers create the database, users, and public shares.
2. The main container reads the prepared database and starts packageR.
3. Kubernetes discards the database when it replaces the Pod. The new Pod
   creates it again from the deployment configuration.

Keep passwords and object-storage credentials in Kubernetes Secrets. Prefer a
workload identity when the object-storage service supports it. Pin the image
to a release tag or digest for a production deployment.

## Example

Create a Secret named `package-r-secrets` with these keys before you apply the
example:

- `admin-password`
- `share-password`
- `aws-access-key-id`
- `aws-secret-access-key`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: package-r
spec:
  replicas: 1
  selector:
    matchLabels:
      app: package-r
  template:
    metadata:
      labels:
        app: package-r
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 100
        fsGroup: 100
      initContainers:
        - name: configure
          image: ghcr.io/versioneer-tech/package-r:latest
          command: ["/package-r"]
          args:
            - config
            - init
            - --address=0.0.0.0
            - --root=my-bucket
            - --stac-browser-url=https://stac-browser.example/external/
          env:
            - name: PACKAGE_R_DATABASE
              value: /state/package-r.db
          volumeMounts:
            - name: state
              mountPath: /state
        - name: create-admin
          image: ghcr.io/versioneer-tech/package-r:latest
          command: ["/package-r"]
          args:
            - users
            - add
            - admin
            - $(ADMIN_PASSWORD)
            - --perm.create=true
            - --perm.delete=true
            - --perm.modify=true
            - --perm.rename=true
          env:
            - name: PACKAGE_R_DATABASE
              value: /state/package-r.db
            - name: ADMIN_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: package-r-secrets
                  key: admin-password
          volumeMounts:
            - name: state
              mountPath: /state
        - name: create-share
          image: ghcr.io/versioneer-tech/package-r:latest
          command: ["/package-r"]
          args:
            - shares
            - add
            - admin
            - public
            - /
            - --password=$(SHARE_PASSWORD)
            - --catalog-name=openaerialmap-assets.parquet
            - '--asset-mappings=[{"from":"s3://data/","to":"."}]'
          env:
            - name: PACKAGE_R_DATABASE
              value: /state/package-r.db
            - name: SHARE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: package-r-secrets
                  key: share-password
          volumeMounts:
            - name: state
              mountPath: /state
      containers:
        - name: package-r
          image: ghcr.io/versioneer-tech/package-r:latest
          env:
            - name: PACKAGE_R_DATABASE
              value: /state/package-r.db
            - name: PACKAGE_R_ADDRESS
              value: 0.0.0.0
            - name: PACKAGE_R_PORT
              value: "8888"
            - name: PACKAGE_R_ROOT
              value: my-bucket
            - name: PACKAGE_R_CORS_ALLOWED_ORIGINS
              value: https://viewer.example.org
            - name: XDG_CACHE_HOME
              value: /cache
            - name: AWS_REGION
              value: us-east-1
            - name: AWS_ACCESS_KEY_ID
              valueFrom:
                secretKeyRef:
                  name: package-r-secrets
                  key: aws-access-key-id
            - name: AWS_SECRET_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: package-r-secrets
                  key: aws-secret-access-key
          ports:
            - name: http
              containerPort: 8888
          readinessProbe:
            httpGet:
              path: /health
              port: http
          livenessProbe:
            httpGet:
              path: /health
              port: http
          volumeMounts:
            - name: state
              mountPath: /state
            - name: cache
              mountPath: /cache
      volumes:
        - name: state
          emptyDir: {}
        - name: cache
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: package-r
spec:
  selector:
    app: package-r
  ports:
    - name: http
      port: 8888
      targetPort: http
```

Kubernetes expands `$(ADMIN_PASSWORD)` and `$(SHARE_PASSWORD)` in the command
arguments. No shell is needed.

Run one replica because each Pod has its own database and session-signing
key. Replacing the Pod ends existing login sessions.

An empty `--password` value creates an unprotected share. An empty or omitted
`--catalog-name` value creates a share without a catalog endpoint. Use a
relative catalog path inside the shared prefix. Use `--asset-mappings` only
when catalog asset URLs do not match paths in that share.

Change the bucket, paths, authentication, permissions, and share definitions
in the init-container commands. Do not add bootstrap logic to the main
container. See [Configuration](configuration.md) for the available settings.

An Ingress normally forwards packageR CORS response headers unchanged. Route
`OPTIONS` requests to packageR, and configure CORS in only one layer to avoid
duplicate or conflicting headers. CORS for presigned object-storage URLs must
still be configured on the object-storage service.
