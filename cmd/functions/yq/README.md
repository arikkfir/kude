# patch

This function patches resources using a YQ expression that serves as both a selector and a patch. 

## Usage

```yaml
apiVersion: kude.kfirs.com/v1alpha1
kind: Pipeline
resources:
  - deployment1.yaml
  - deployment2.yaml
pipeline:
  - image: ghcr.io/arikkfir/kude/functions/yq
    config:
      expr: |-
        .[] |= with(
          select(.apiVersion == "apps/v1" and 
                 .kind == "Deployment" and 
                 .metadata.labels.app == "my-app");
          .spec.template.spec.tolerations += { "key": "workload-nodes", "operator": "Exists" }
        )
```

This will add the given toleration to any `Deployment` object matching the label selector `app=my-app`.

You can update multiple properties in matching objects like so:

```yaml
apiVersion: kude.kfirs.com/v1alpha1
kind: Pipeline
resources:
  - deployment1.yaml
  - deployment2.yaml
pipeline:
  - image: ghcr.io/arikkfir/kude/functions/yq
    config:
      expr: |-
        .[] |= with(
          select(.apiVersion == "apps/v1" and 
                 .kind == "Deployment" and 
                 .metadata.name == "first-deployment");
          .metadata.labels.app = "first" | 
          .spec.replicas = 3
        )
```

This will add the label `app` with the vaue `first` to the deployment called `first-deployment`, and will also set the
`spec/replicas` field to `3`.
