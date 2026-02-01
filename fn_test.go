package main

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

func TestRunFunction(t *testing.T) {
	type args struct {
		ctx context.Context
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"RequestEnvironmentConfigs": {
			reason: "The Function should request the necessary EnvironmentConfigs",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "test.crossplane.io/v1alpha1",
								"kind": "XR",
								"metadata": {
									"name": "my-xr"
								},
								"spec": {
									"existingEnvSelectorLabel": "someMoreBar"
								}
							}`),
						},
					},
					Input: resource.MustStructJSON(`{
						"apiVersion": "template.fn.crossplane.io/v1beta1",
						"kind": "Input",
						"spec": {
							"environmentConfigs": [
								{
									"type": "Reference",
									"ref": {
										"name": "my-env-config"
									}
								},
								{
									"type": "Reference",
									"ref": {
										"name": "my-second-env-config"
									}
								},
								{
									"type": "Selector",
									"selector": {
										"mode": "Multiple",
										"matchLabels": [
											{
												"type": "Value",
												"key": "foo",
												"value": "bar"
											}
										]
									}
								},
								{
									"type": "Selector",
									"selector": {
										"mode": "Single",
										"matchLabels": [
											{
												"key": "someMoreFoo",
												"valueFromFieldPath": "spec.missingEnvSelectorLabel",
												"fromFieldPathPolicy": "Optional"
											}
										]
									}
								},
								{
									"type": "Selector",
									"selector": {
										"mode": "Single",
										"matchLabels": [
											{
												"key": "someMoreFoo",
												"valueFromFieldPath": "spec.existingEnvSelectorLabel",
												"fromFieldPathPolicy": "Required"
											}
										]
									}
								}
							]
						}
					}`),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:    &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{},
					Requirements: &fnv1.Requirements{
						Resources: map[string]*fnv1.ResourceSelector{
							"environment-config-0": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "my-env-config",
								},
							},
							"environment-config-1": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "my-second-env-config",
								},
							},
							"environment-config-2": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1.MatchLabels{
										Labels: map[string]string{
											"foo": "bar",
										},
									},
								},
							},
							// environment-config-3 is not requested because it was optional
							"environment-config-4": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1.MatchLabels{
										Labels: map[string]string{
											"someMoreFoo": "someMoreBar",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"RequestEnvironmentConfigsFound": {
			reason: "The Function should request the necessary EnvironmentConfigs even if they are already present in the request",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "test.crossplane.io/v1alpha1",
								"kind": "XR",
								"metadata": {
									"name": "my-xr"
								},
								"spec": {
									"existingEnvSelectorLabel": "someMoreBar"
								}
							}`),
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"environment-config-0": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1beta1",
									"kind": "EnvironmentConfig",
									"metadata": {
										"name": "my-env-config"
									},
									"data": {
										"firstKey": "firstVal",
										"secondKey": "secondVal"
									}
								}`),
								},
							},
						},
						"environment-config-1": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1beta1",
									"kind": "EnvironmentConfig",
									"metadata": {
										"name": "my-second-env-config"
									},
									"data": {
										"secondKey": "secondVal-ok",
										"thirdKey": "thirdVal"
									}
								}`),
								},
							},
						},
						"environment-config-2": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1beta1",
									"kind": "EnvironmentConfig",
									"metadata": {
										"name": "my-third-env-config-b"
									},
									"data": {
										"fourthKey": "fourthVal-b"
									}
								}`),
								},
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1beta1",
									"kind": "EnvironmentConfig",
									"metadata": {
										"name": "my-third-env-config-a"
									},
									"data": {
										"fourthKey": "fourthVal-a"
									}
								}`),
								},
							},
						},
						"environment-config-3": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1beta1",
									"kind": "EnvironmentConfig",
									"metadata": {
										"name": "my-third-env-config"
									},
									"data": {
										"fifthKey": "fifthVal"
									}
								}`),
								},
							},
						},
						"environment-config-4": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1beta1",
									"kind": "EnvironmentConfig",
									"metadata": {
										"name": "my-fourth-env-config"
									},
									"data": {
										"sixthKey": "sixthVal"
									}
								}`),
								},
							},
						},
					},
					Input: resource.MustStructJSON(`{
						"apiVersion": "template.fn.crossplane.io/v1beta1",
						"kind": "Input",
						"spec": {
							"environmentConfigs": [
								{
									"type": "Reference",
									"ref": {
										"name": "my-env-config"
									}
								},
								{
									"type": "Reference",
									"ref": {
										"name": "my-second-env-config"
									}
								},
								{
									"type": "Selector",
									"selector": {
										"mode": "Multiple",
										"matchLabels": [
											{
												"type": "Value",
												"key": "foo",
												"value": "bar"
											}
										]
									}
								},
								{
									"type": "Selector",
									"selector": {
										"mode": "Single",
										"matchLabels": [
											{
												"key": "someMoreFoo",
												"valueFromFieldPath": "spec.missingEnvSelectorLabel",
												"fromFieldPathPolicy": "Optional"
											}
										]
									}
								},
								{
									"type": "Selector",
									"selector": {
										"mode": "Single",
										"matchLabels": [
											{
												"key": "someMoreFoo",
												"valueFromFieldPath": "spec.existingEnvSelectorLabel",
												"fromFieldPathPolicy": "Required"
											}
										]
									}
								}
							]
						}
					}`),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:    &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{},
					Requirements: &fnv1.Requirements{
						Resources: map[string]*fnv1.ResourceSelector{
							"environment-config-0": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "my-env-config",
								},
							},
							"environment-config-1": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "my-second-env-config",
								},
							},
							"environment-config-2": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1.MatchLabels{
										Labels: map[string]string{
											"foo": "bar",
										},
									},
								},
							},
							// environment-config-3 is not requested because it was optional
							"environment-config-4": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1.MatchLabels{
										Labels: map[string]string{
											"someMoreFoo": "someMoreBar",
										},
									},
								},
							},
						},
					},
					Context: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							FunctionContextKeyEnvironment: structpb.NewStructValue(resource.MustStructJSON(`{
								"apiVersion": "internal.crossplane.io/v1alpha1",
								"kind": "Environment",
								"firstKey": "firstVal",
								"secondKey": "secondVal-ok",
								"thirdKey": "thirdVal",
								"fourthKey": "fourthVal-b",
								"fifthKey": "fifthVal",
								"sixthKey": "sixthVal"
							}`)),
						},
					},
				},
			},
		},
		"RequestEnvironmentConfigsNotFoundRequired": {
			reason: "The Function should return fatal if a required EnvironmentConfig is not found",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "test.crossplane.io/v1alpha1",
								"kind": "XR",
								"metadata": {
									"name": "my-xr"
								}
							}`),
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"environment-config-0": {
							Items: []*fnv1.Resource{},
						},
					},
					Input: resource.MustStructJSON(`{
						"apiVersion": "template.fn.crossplane.io/v1beta1",
						"kind": "Input",
						"spec": {
							"environmentConfigs": [
								{
									"type": "Reference",
									"ref": {
										"name": "my-env-config"
									}
								}
							]
						}
					}`),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Target:   ptr.To(fnv1.Target_TARGET_COMPOSITE),
						},
					},
					Requirements: &fnv1.Requirements{
						Resources: map[string]*fnv1.ResourceSelector{
							"environment-config-0": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "my-env-config",
								},
							},
						},
					},
				},
			},
		},
		"SelectorWithOptionalFieldPathNotProvided": {
			reason: "The Function should gracefully skip selectors with optional field paths when the environment config is not provided in extraResources",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "test.crossplane.io/v1alpha1",
								"kind": "XR",
								"metadata": {
									"name": "my-xr"
								},
								"spec": {
									"presentField": "value"
								}
							}`),
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"environment-config-0": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
										"apiVersion": "apiextensions.crossplane.io/v1beta1",
										"kind": "EnvironmentConfig",
										"metadata": {
											"name": "base-env-config"
										},
										"data": {
											"baseKey": "baseVal"
										}
									}`),
								},
							},
						},
						// environment-config-1 is NOT provided (optional field doesn't exist)
					},
					Input: resource.MustStructJSON(`{
						"apiVersion": "template.fn.crossplane.io/v1beta1",
						"kind": "Input",
						"spec": {
							"environmentConfigs": [
								{
									"type": "Reference",
									"ref": {
										"name": "base-env-config"
									}
								},
								{
									"type": "Selector",
									"selector": {
										"mode": "Multiple",
										"minMatch": 0,
										"maxMatch": 1,
										"matchLabels": [
											{
												"key": "epd",
												"valueFromFieldPath": "spec.epd.name",
												"fromFieldPathPolicy": "Optional"
											}
										]
									}
								}
							]
						}
					}`),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:    &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{},
					Requirements: &fnv1.Requirements{
						Resources: map[string]*fnv1.ResourceSelector{
							"environment-config-0": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "base-env-config",
								},
							},
							// environment-config-1 is not in requirements because optional field doesn't exist
						},
					},
					Context: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							FunctionContextKeyEnvironment: structpb.NewStructValue(resource.MustStructJSON(`{
								"apiVersion": "internal.crossplane.io/v1alpha1",
								"kind": "Environment",
								"baseKey": "baseVal"
							}`)),
						},
					},
				},
			},
		},
		"SelectorSingleModeWithOptionalFieldPathNotProvided": {
			reason: "Single mode should gracefully skip when optional field path doesn't exist (per documentation: 'if any others exist')",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "test.crossplane.io/v1alpha1",
								"kind": "XR",
								"metadata": {
									"name": "my-xr"
								},
								"spec": {
									"presentField": "value"
								}
							}`),
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"environment-config-0": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
										"apiVersion": "apiextensions.crossplane.io/v1beta1",
										"kind": "EnvironmentConfig",
										"metadata": {
											"name": "base-env-config"
										},
										"data": {
											"baseKey": "baseVal"
										}
									}`),
								},
							},
						},
						// environment-config-1 is NOT provided (optional field doesn't exist)
					},
					Input: resource.MustStructJSON(`{
						"apiVersion": "template.fn.crossplane.io/v1beta1",
						"kind": "Input",
						"spec": {
							"environmentConfigs": [
								{
									"type": "Reference",
									"ref": {
										"name": "base-env-config"
									}
								},
								{
									"type": "Selector",
									"selector": {
										"mode": "Single",
										"matchLabels": [
											{
												"key": "epd",
												"valueFromFieldPath": "spec.epd.name",
												"fromFieldPathPolicy": "Optional"
											}
										]
									}
								}
							]
						}
					}`),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:    &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{},
					Requirements: &fnv1.Requirements{
						Resources: map[string]*fnv1.ResourceSelector{
							"environment-config-0": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "base-env-config",
								},
							},
							// environment-config-1 is not in requirements because optional field doesn't exist
						},
					},
					Context: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							FunctionContextKeyEnvironment: structpb.NewStructValue(resource.MustStructJSON(`{
								"apiVersion": "internal.crossplane.io/v1alpha1",
								"kind": "Environment",
								"baseKey": "baseVal"
							}`)),
						},
					},
				},
			},
		},
		"MergeEnvironmentConfigs": {
			reason: "The Function should merge the provided EnvironmentConfigs",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Context: resource.MustStructJSON(`{
						"` + FunctionContextKeyEnvironment + `": {
							"apiVersion": "internal.crossplane.io/v1alpha1",
							"kind": "Environment",
							"a": "only-from-input",
							"e": "overridden-from-input-ok",
							"f": "overridden-from-env-config-1",
							"g": "overridden-from-env-config-2"
						}
					}`),
					Input: resource.MustStructJSON(`{
						"apiVersion": "template.fn.crossplane.io/v1beta1",
						"kind": "Input",
						"spec": {
							"defaultData": {
								"b": "only-from-default",
								"e": "overridden-from-input",
								"f": "overridden-from-env-config-2"
							},
							"environmentConfigs": [
								{
									"type": "Reference",
									"ref": {
										"name": "my-env-config"
									}
								},
								{
									"type": "Reference",
									"ref": {
										"name": "my-second-env-config"
									}
								}
							]
						}
					}`),
					RequiredResources: map[string]*fnv1.Resources{
						"environment-config-0": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1beta1",
									"kind": "EnvironmentConfig",
									"metadata": {
										"name": "my-env-config"
									},
									"data": {
										"c": "only-from-env-config-1",
										"f": "overridden-from-env-config-1-ok",
										"h": "override-from-env-config-1"
									}
								}`),
								},
							},
						},
						"environment-config-1": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1beta1",
									"kind": "EnvironmentConfig",
									"metadata": {
										"name": "my-second-env-config"
									},
									"data": {
										"d": "only-from-env-config-1",
										"g": "overridden-from-env-config-2-ok",
										"h": "override-from-env-config-1-ok"
									}
								}`),
								},
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:    &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{},
					Requirements: &fnv1.Requirements{
						Resources: map[string]*fnv1.ResourceSelector{
							"environment-config-0": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "my-env-config",
								},
							},
							"environment-config-1": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "my-second-env-config",
								},
							},
						},
					},
					Context: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							FunctionContextKeyEnvironment: structpb.NewStructValue(resource.MustStructJSON(`{
								"apiVersion": "internal.crossplane.io/v1alpha1",
								"kind": "Environment",
								"a": "only-from-input",
								"b": "only-from-default",
								"c": "only-from-env-config-1",
								"d": "only-from-env-config-1",
								"e": "overridden-from-input-ok",
								"f": "overridden-from-env-config-1-ok",
								"g": "overridden-from-env-config-2-ok",
								"h": "override-from-env-config-1-ok"
							}`)),
						},
					},
				},
			},
		},
		"ToFieldPath": {
			reason: "The Function should load into the specified toFieldPath",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "template.fn.crossplane.io/v1beta1",
						"kind": "Input",
						"spec": {
							"defaultData": {
								"a": "from-default"
							},
							"environmentConfigs": [
								{
									"type": "Reference",
									"ref": {
										"name": "foo"
									},
									"toFieldPath": "foo"
								},
								{
									"type": "Selector",
									"selector": {
										"mode": "Multiple",
										"matchLabels": [
											{
												"type": "Value",
												"key": "foo",
												"value": "bar"
											}
										]
									},
									"toFieldPath": "foo.bar"
								}
							]
						}
					}`),
					RequiredResources: map[string]*fnv1.Resources{
						"environment-config-0": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1beta1",
									"kind": "EnvironmentConfig",
									"metadata": {
										"name": "foo"
									},
									"data": {
										"a": "from-foo"
									}
								}`),
								},
							},
						},
						"environment-config-1": {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1beta1",
									"kind": "EnvironmentConfig",
									"metadata": {
										"name": "first"
									},
									"data": {
										"a": "from-label-select-first"
									}
								}`),
								},
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1beta1",
									"kind": "EnvironmentConfig",
									"metadata": {
										"name": "second"
									},
									"data": {
										"b": "from-label-select-second"
									}
								}`),
								},
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:    &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{},
					Requirements: &fnv1.Requirements{
						Resources: map[string]*fnv1.ResourceSelector{
							"environment-config-0": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchName{
									MatchName: "foo",
								},
							},
							"environment-config-1": {
								ApiVersion: "apiextensions.crossplane.io/v1beta1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1.MatchLabels{
										Labels: map[string]string{
											"foo": "bar",
										},
									},
								},
							},
						},
					},
					Context: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							FunctionContextKeyEnvironment: structpb.NewStructValue(resource.MustStructJSON(`{
								"apiVersion": "internal.crossplane.io/v1alpha1",
								"kind": "Environment",
								"a": "from-default",
								"foo": {
									"a": "from-foo",
									"bar": {
										"a": "from-label-select-first",
										"b": "from-label-select-second"
									}
								}
							}`)),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			diff := cmp.Diff(tc.want.rsp, rsp, cmpopts.AcyclicTransformer("toJsonWithoutResultMessages", func(r *fnv1.RunFunctionResponse) []byte {
				// We don't care about messages.
				// cmptopts.IgnoreField wasn't working with protocmp.Transform
				// We can't split this to another transformer as
				// transformers are applied not in order but as soon as they
				// match the type, which are walked from the root (RunFunctionResponse).
				for _, result := range r.GetResults() {
					result.Message = ""
				}
				out, err := protojson.Marshal(r)
				if err != nil {
					t.Fatalf("cannot marshal %T to JSON: %s", r, err)
				}
				return out
			}))
			if diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func resourceWithFieldPathValue(path string, value any) resource.Required {
	u := unstructured.Unstructured{
		Object: map[string]any{},
	}
	err := fieldpath.Pave(u.Object).SetValue(path, value)
	if err != nil {
		panic(err)
	}
	return resource.Required{
		Resource: &u,
	}
}

func TestSortRequiredByFieldPath(t *testing.T) {
	type args struct {
		requiredResources []resource.Required
		path              string
	}
	type want struct {
		requiredResources []resource.Required
		err               error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SortByString": {
			reason: "The Function should sort the required resources by the string value at the specified field path",
			args: args{
				requiredResources: []resource.Required{
					resourceWithFieldPathValue("metadata.name", "c"),
					resourceWithFieldPathValue("metadata.name", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
				},
				path: "metadata.name",
			},
			want: want{
				requiredResources: []resource.Required{
					resourceWithFieldPathValue("metadata.name", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
					resourceWithFieldPathValue("metadata.name", "c"),
				},
			},
		},
		"SortByInt": {
			reason: "The Function should sort the required resources by the int value at the specified field path",
			args: args{
				requiredResources: []resource.Required{
					resourceWithFieldPathValue("data.someInt", 3),
					resourceWithFieldPathValue("data.someInt", 1),
					resourceWithFieldPathValue("data.someInt", 2),
				},
				path: "data.someInt",
			},
			want: want{
				requiredResources: []resource.Required{
					resourceWithFieldPathValue("data.someInt", 1),
					resourceWithFieldPathValue("data.someInt", 2),
					resourceWithFieldPathValue("data.someInt", 3),
				},
			},
		},
		"SortByFloat": {
			reason: "The Function should sort the required resources by the float value at the specified field path",
			args: args{
				requiredResources: []resource.Required{
					resourceWithFieldPathValue("data.someFloat", 1.3),
					resourceWithFieldPathValue("data.someFloat", 1.1),
					resourceWithFieldPathValue("data.someFloat", 1.2),
					resourceWithFieldPathValue("data.someFloat", 1.4),
				},
				path: "data.someFloat",
			},
			want: want{
				requiredResources: []resource.Required{
					resourceWithFieldPathValue("data.someFloat", 1.1),
					resourceWithFieldPathValue("data.someFloat", 1.2),
					resourceWithFieldPathValue("data.someFloat", 1.3),
					resourceWithFieldPathValue("data.someFloat", 1.4),
				},
			},
		},
		"InconsistentTypeSortByInt": {
			reason: "The Function should sort the required resources by the int value at the specified field path",
			args: args{
				requiredResources: []resource.Required{
					resourceWithFieldPathValue("data.someInt", 3),
					resourceWithFieldPathValue("data.someInt", 1),
					resourceWithFieldPathValue("data.someInt", "2"),
				},
				path: "data.someInt",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"EmptyPath": {
			reason: "The Function should return an error if the path is empty",
			args: args{
				requiredResources: []resource.Required{
					resourceWithFieldPathValue("metadata.name", "c"),
					resourceWithFieldPathValue("metadata.name", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
				},
				path: "",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"InvalidPathAll": {
			reason: "The Function should return no error if the path is invalid for all resources",
			args: args{
				requiredResources: []resource.Required{
					resourceWithFieldPathValue("metadata.name", "c"),
					resourceWithFieldPathValue("metadata.name", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
				},
				path: "metadata.invalid",
			},
			want: want{
				requiredResources: []resource.Required{
					resourceWithFieldPathValue("metadata.name", "c"),
					resourceWithFieldPathValue("metadata.name", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
				},
			},
		},
		"InvalidPathSome": {
			reason: "The Function should return no error if the path is invalid for some resources, just use the rest of the resources zero value",
			args: args{
				requiredResources: []resource.Required{
					resourceWithFieldPathValue("metadata.name", "c"),
					resourceWithFieldPathValue("metadata.invalid", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
				},
				path: "metadata.name",
			},
			want: want{
				requiredResources: []resource.Required{
					resourceWithFieldPathValue("metadata.invalid", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
					resourceWithFieldPathValue("metadata.name", "c"),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := sortRequiredByFieldPath(tc.args.requiredResources, tc.args.path)
			if diff := cmp.Diff(tc.want.err, got, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\n(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			if tc.want.err != nil {
				return
			}
			if diff := cmp.Diff(tc.want.requiredResources, tc.args.requiredResources); diff != "" {
				t.Errorf("%s\n(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
