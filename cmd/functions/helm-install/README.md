# helm-install

This function generates a `Job` which will install a Helm chart when executed.

## Usage

```yaml
apiVersion: kude.kfirs.com/v1alpha2
kind: Pipeline
resources:
  - deployment.yaml
steps:
  - image: ghcr.io/arikkfir/kude/functions/helm-install
    config:
      name: my-chart-1
      chart: my-chart
      repo: https://some.repo.com # optional
      job: # optional
        backoffLimit: 2
        restartPolicy: OnFailure
        serviceAccountName: kude-service-account
      helm: # optional
        version: 3.9.3
      flags: # optional
        - --create-namespace
        - --set cpu=2
        - --values values.yaml # assume it contains just "foo: bar"
    mounts:
      - my-chart-values.yaml:values.yaml
```

This would generate a YAML running a `Job` that would run `helm instal ...`, like so:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-chart-1-values
data:
  values.yaml: |-
    foo: bar
---
apiVersion: batch/v1
kind: Job
metadata:
  name: kude-helm-install-my-chart-1
spec:
  backoffLimit: 2
  template:
    spec:
      containers:
        - args:
            - install
            - my-chart-1
            - my-chart
            - --repo=https://some.repo.com
            - --create-namespace
            - --set cpu=2
            - --values values.yaml
          image: alpine/helm:3.9.3
          imagePullPolicy: IfNotPresent
          name: helm-install
          workingDir: /work
          volumeMounts:
            - name: my-chart-values
              mountPath: /work
      volumes:
        - name: my-chart-values
          configMap:
            name: my-chart-1-values  
```

The values in the `data` map are base64 encoded as required by Kubernetes (this is not encryption!).
