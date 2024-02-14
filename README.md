# function-environment-configs

[![CI](https://github.com/crossplane-contrib/function-environment-configs/actions/workflows/ci.yml/badge.svg)](https://github.com/crossplane-contrib/function-environment-configs/actions/workflows/ci.yml) ![GitHub release (latest SemVer)](https://img.shields.io/github/release/crossplane-contrib/function-environment-configs)

> [!WARNING]
> This function is under active development and is not yet stable.
> This function relies on functionalities only available in Crossplane 1.15 and
> later. It will not work with earlier versions.

This [composition function][docs-functions] allows you to request
`EnvironmentConfigs`, merge them in the requested order and return inject the
computed `environment` into the `Context` at a well-known key,
`apiextensions.crossplane.io/environment`, so that other functions such as
[function-patch-and-transform] can access it.

The following paths from native `Resource` Compositions can be just moved to the
input of this function:
- `spec.environment.environmentConfigs` -> `spec.environmentConfigs`
- `spec.environment.defaultData` -> `spec.defaultData`
- `spec.environment.policy.resolution` -> `spec.policy.resolution`

`spec.environment.policy.resolve` is not supported yet, which results
effectively in the same behavior as `Always`.

Here's an example:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
   name: function-environment-configs
spec:
   compositeTypeRef:
      apiVersion: example.crossplane.io/v1
      kind: XR
   mode: Pipeline
   pipeline:
      - step: environmentConfigs
        functionRef:
           name: function-environment-configs
        input:
           apiVersion: environmentconfigs.fn.crossplane.io/v1beta1
           kind: Input
           spec:
              environmentConfigs:
                 - type: Reference
                   ref:
                      name: example-config
      - step: go-templating
        functionRef:
           name: function-go-templating
        input:
           apiVersion: gotemplating.fn.crossplane.io/v1beta1
           kind: GoTemplate
           source: Inline
           inline:
              template: |
                 ---
                 apiVersion: example.crossplane.io/v1
                 kind: XR
                 status:
                   fromEnv: {{ index .context "apiextensions.crossplane.io/environment" "complex" "c" "d" }}
```

## Using this function

See the [example](example) directory for examples that you can run locally using
the Crossplane CLI:

```shell
$ crossplane beta render \
  --extra-resources example/environmentConfigs.yaml \
  --include-context \
  example/xr.yaml example/composition.yaml example/functions.yaml
```

To validate the rendered output, you can use the `crossplane beta validate` command:
```shell
$ crossplane beta render \
  --extra-resources example/environmentConfigs.yaml \
  --include-full-xr \
  example/xr.yaml example/composition.yaml example/functions.yaml | crossplane beta validate example -
```

See the [composition functions documentation][docs-functions] to learn more
about `crossplane beta render`.

## Developing this function

This function uses [Go][go], [Docker][docker], and the [Crossplane CLI][cli] to
build functions.

```shell
# Run code generation - see input/generate.go
$ go generate ./...

# Run tests - see fn_test.go
$ go test ./...

# Build the function's runtime image - see Dockerfile
$ docker build . --tag=runtime

# Build a function package - see package/crossplane.yaml
$ crossplane xpkg build -f package --embed-runtime-image=runtime
```

[docs-functions]: https://docs.crossplane.io/v1.14/concepts/composition-functions/
[bsr]: https://buf.build/crossplane/crossplane/docs/main:apiextensions.fn.proto.v1beta1#apiextensions.fn.proto.v1beta1.RunFunctionRequest
[go]: https://go.dev
[docker]: https://www.docker.com
[cli]: https://docs.crossplane.io/latest/cli
[function-patch-and-transform]: https://github.com/crossplane-contrib/function-patch-and-transform
