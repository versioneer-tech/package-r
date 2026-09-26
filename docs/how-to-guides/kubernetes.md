# Run packageR on Kubernetes

The packageR container starts the application directly. Prepare its runtime
database before the main container starts. An init container and an
`emptyDir` volume make this setup explicit:

1. The init container creates a new database, users, and public shares.
2. The main container reads the prepared database and starts packageR.
3. Kubernetes discards the database when it replaces the Pod. The next init
   container creates it again from the deployment configuration.

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
        - name: prepare-database
          image: ghcr.io/versioneer-tech/package-r:latest
          command: ["/bin/sh", "-ec"]
          args:
            - |
              database=/state/package-r.db
              work_database=/state/package-r.db.new
              rm -f "$work_database"
              export PACKAGE_R_DATABASE="$work_database"

              /package-r config init \
                --address=0.0.0.0 \
                --port=8888 \
                --root=my-bucket \
                --stac-browser-url=https://browser.moregeo.it/external/ \
                --auth.method=json \
                --signup=false \
                --create-user-dir=false \
                --disable-exec=true \
                --disable-preview-resize=true \
                --disable-thumbnails=true \
                --disable-type-detection-by-header=true \
                --scope=/ \
                --perm.create=false \
                --perm.delete=false \
                --perm.modify=false \
                --perm.rename=false

              /package-r users add admin "$ADMIN_PASSWORD" \
                --scope=/ \
                --perm.create=true \
                --perm.delete=true \
                --perm.modify=true \
                --perm.rename=true

              /package-r shares add admin public /catalog-sample \
                --password="$SHARE_PASSWORD" \
                --catalog-name=catalog.parquet \
                --asset-mappings='[{"from":"s3://data/","to":"."}]'

              mv "$work_database" "$database"
          env:
            - name: ADMIN_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: package-r-secrets
                  key: admin-password
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

The main container can update runtime state in the database. These changes
last for the life of the Pod only. A replacement Pod gets a new database from
its init container and existing login sessions end. Run one replica because
each Pod has its own database and session-signing key.

An empty `--password` value creates an unprotected share. An empty or omitted
`--catalog-name` value creates a share without a catalog endpoint. Use a
relative catalog path inside the shared prefix. Use `--asset-mappings` only
when catalog asset URLs do not match paths in that share.

Change the bucket, paths, authentication, permissions, and share definitions
in the init-container command. Do not add bootstrap logic to the main
container. See [Configuration](configuration.md) for the available settings.
