# Example manifests

You can run your function locally and test it using `crossplane beta render`
with these example manifests.

```shell
# Run the function locally
$ go run . --insecure --debug
```

```shell
# Then, in another terminal, call it with these example manifests
$ crossplane beta render \
  --extra-resources example/environmentConfigs.yaml \
  --include-context \
  example/xr.yaml example/composition.yaml example/functions.yaml
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
  fromEnv: e
---
apiVersion: render.crossplane.io/v1beta1
kind: Context
fields:
  apiextensions.crossplane.io/environment:
    kind: Environment
    apiVersion: internal.crossplane.io/v1alpha1
    complex:
      a: b
      c:
        d: e
        f: "1"
```
