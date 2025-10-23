# Example manifests

You can run your function locally and test it using `crossplane beta render`
with these example manifests.

```shell
# Run the function locally
$ go run . --insecure --debug
```

```shell
# Then, in another terminal, call it with these example manifests
$ crossplane render \
  --extra-resources match_label_multiple/environmentConfigs.yaml \
  --include-context \
  match_label_multiple/xr.yaml match_label_multiple/composition.yaml match_label_multiple/functions.yaml
---
apiVersion: example.crossplane.io/v1
kind: XR
metadata:
  name: example-xr
status:
  conditions:
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    reason: Available
    status: "True"
    type: Ready
  fromEnvOne: e
  fromEnvTwo: k
---
apiVersion: render.crossplane.io/v1beta1
fields:
  apiextensions.crossplane.io/environment:
    apiVersion: internal.crossplane.io/v1alpha1
    kind: Environment
    multiple:
      a: b
      c:
        d: e
        f: "1"
      g: h
      i:
        j: k
kind: Context
```
