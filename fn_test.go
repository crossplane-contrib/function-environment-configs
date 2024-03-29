package main

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

func TestRunFunction(t *testing.T) {

	type args struct {
		ctx context.Context
		req *fnv1beta1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1beta1.RunFunctionResponse
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
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
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
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Requirements: &fnv1beta1.Requirements{
						ExtraResources: map[string]*fnv1beta1.ResourceSelector{
							"environment-config-0": {
								ApiVersion: "apiextensions.crossplane.io/v1alpha1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1beta1.ResourceSelector_MatchName{
									MatchName: "my-env-config",
								},
							},
							"environment-config-1": {
								ApiVersion: "apiextensions.crossplane.io/v1alpha1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1beta1.ResourceSelector_MatchName{
									MatchName: "my-second-env-config",
								},
							},
							"environment-config-2": {
								ApiVersion: "apiextensions.crossplane.io/v1alpha1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1beta1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1beta1.MatchLabels{
										Labels: map[string]string{
											"foo": "bar",
										},
									},
								},
							},
							// environment-config-3 is not requested because it was optional
							"environment-config-4": {
								ApiVersion: "apiextensions.crossplane.io/v1alpha1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1beta1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1beta1.MatchLabels{
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
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
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
					ExtraResources: map[string]*fnv1beta1.Resources{
						"environment-config-0": {
							Items: []*fnv1beta1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1alpha1",
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
							Items: []*fnv1beta1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1alpha1",
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
							Items: []*fnv1beta1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1alpha1",
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
									"apiVersion": "apiextensions.crossplane.io/v1alpha1",
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
							Items: []*fnv1beta1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1alpha1",
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
							Items: []*fnv1beta1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1alpha1",
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
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Requirements: &fnv1beta1.Requirements{
						ExtraResources: map[string]*fnv1beta1.ResourceSelector{
							"environment-config-0": {
								ApiVersion: "apiextensions.crossplane.io/v1alpha1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1beta1.ResourceSelector_MatchName{
									MatchName: "my-env-config",
								},
							},
							"environment-config-1": {
								ApiVersion: "apiextensions.crossplane.io/v1alpha1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1beta1.ResourceSelector_MatchName{
									MatchName: "my-second-env-config",
								},
							},
							"environment-config-2": {
								ApiVersion: "apiextensions.crossplane.io/v1alpha1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1beta1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1beta1.MatchLabels{
										Labels: map[string]string{
											"foo": "bar",
										},
									},
								},
							},
							// environment-config-3 is not requested because it was optional
							"environment-config-4": {
								ApiVersion: "apiextensions.crossplane.io/v1alpha1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1beta1.ResourceSelector_MatchLabels{
									MatchLabels: &fnv1beta1.MatchLabels{
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
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "test.crossplane.io/v1alpha1",
								"kind": "XR",
								"metadata": {
									"name": "my-xr"
								}
							}`),
						},
					},
					ExtraResources: map[string]*fnv1beta1.Resources{
						"environment-config-0": {
							Items: []*fnv1beta1.Resource{},
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
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{
						{
							Severity: fnv1beta1.Severity_SEVERITY_FATAL,
						},
					},
					Requirements: &fnv1beta1.Requirements{
						ExtraResources: map[string]*fnv1beta1.ResourceSelector{
							"environment-config-0": {
								ApiVersion: "apiextensions.crossplane.io/v1alpha1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1beta1.ResourceSelector_MatchName{
									MatchName: "my-env-config",
								},
							},
						},
					},
				},
			},
		},
		"MergeEnvironmentConfigs": {
			reason: "The Function should merge the provided EnvironmentConfigs",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
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
					ExtraResources: map[string]*fnv1beta1.Resources{
						"environment-config-0": {
							Items: []*fnv1beta1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1alpha1",
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
							Items: []*fnv1beta1.Resource{
								{
									Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.crossplane.io/v1alpha1",
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
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta:    &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1beta1.Result{},
					Requirements: &fnv1beta1.Requirements{
						ExtraResources: map[string]*fnv1beta1.ResourceSelector{
							"environment-config-0": {
								ApiVersion: "apiextensions.crossplane.io/v1alpha1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1beta1.ResourceSelector_MatchName{
									MatchName: "my-env-config",
								},
							},
							"environment-config-1": {
								ApiVersion: "apiextensions.crossplane.io/v1alpha1",
								Kind:       "EnvironmentConfig",
								Match: &fnv1beta1.ResourceSelector_MatchName{
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			diff := cmp.Diff(tc.want.rsp, rsp, cmpopts.AcyclicTransformer("toJsonWithoutResultMessages", func(r *fnv1beta1.RunFunctionResponse) []byte {
				// We don't care about messages.
				// cmptopts.IgnoreField wasn't working with protocmp.Transform
				// We can't split this to another transformer as
				// transformers are applied not in order but as soon as they
				// match the type, which are walked from the root (RunFunctionResponse).
				for _, result := range r.Results {
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

func resourceWithFieldPathValue(path string, value any) resource.Extra {
	u := unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	err := fieldpath.Pave(u.Object).SetValue(path, value)
	if err != nil {
		panic(err)
	}
	return resource.Extra{
		Resource: &u,
	}
}

func TestSortExtrasByFieldPath(t *testing.T) {
	type args struct {
		extras []resource.Extra
		path   string
	}
	type want struct {
		extras []resource.Extra
		err    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SortByString": {
			reason: "The Function should sort the Extras by the string value at the specified field path",
			args: args{
				extras: []resource.Extra{
					resourceWithFieldPathValue("metadata.name", "c"),
					resourceWithFieldPathValue("metadata.name", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
				},
				path: "metadata.name",
			},
			want: want{
				extras: []resource.Extra{
					resourceWithFieldPathValue("metadata.name", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
					resourceWithFieldPathValue("metadata.name", "c"),
				},
			},
		},
		"SortByInt": {
			reason: "The Function should sort the Extras by the int value at the specified field path",
			args: args{
				extras: []resource.Extra{
					resourceWithFieldPathValue("data.someInt", 3),
					resourceWithFieldPathValue("data.someInt", 1),
					resourceWithFieldPathValue("data.someInt", 2),
				},
				path: "data.someInt",
			},
			want: want{
				extras: []resource.Extra{
					resourceWithFieldPathValue("data.someInt", 1),
					resourceWithFieldPathValue("data.someInt", 2),
					resourceWithFieldPathValue("data.someInt", 3),
				},
			},
		},
		"SortByFloat": {
			reason: "The Function should sort the Extras by the float value at the specified field path",
			args: args{
				extras: []resource.Extra{
					resourceWithFieldPathValue("data.someFloat", 1.3),
					resourceWithFieldPathValue("data.someFloat", 1.1),
					resourceWithFieldPathValue("data.someFloat", 1.2),
					resourceWithFieldPathValue("data.someFloat", 1.4),
				},
				path: "data.someFloat",
			},
			want: want{
				extras: []resource.Extra{
					resourceWithFieldPathValue("data.someFloat", 1.1),
					resourceWithFieldPathValue("data.someFloat", 1.2),
					resourceWithFieldPathValue("data.someFloat", 1.3),
					resourceWithFieldPathValue("data.someFloat", 1.4),
				},
			},
		},
		"InconsistentTypeSortByInt": {
			reason: "The Function should sort the Extras by the int value at the specified field path",
			args: args{
				extras: []resource.Extra{
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
				extras: []resource.Extra{
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
				extras: []resource.Extra{
					resourceWithFieldPathValue("metadata.name", "c"),
					resourceWithFieldPathValue("metadata.name", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
				},
				path: "metadata.invalid",
			},
			want: want{
				extras: []resource.Extra{
					resourceWithFieldPathValue("metadata.name", "c"),
					resourceWithFieldPathValue("metadata.name", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
				},
			},
		},
		"InvalidPathSome": {
			reason: "The Function should return no error if the path is invalid for some resources, just use the rest of the resources zero value",
			args: args{
				extras: []resource.Extra{
					resourceWithFieldPathValue("metadata.name", "c"),
					resourceWithFieldPathValue("metadata.invalid", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
				},
				path: "metadata.name",
			},
			want: want{
				extras: []resource.Extra{
					resourceWithFieldPathValue("metadata.invalid", "a"),
					resourceWithFieldPathValue("metadata.name", "b"),
					resourceWithFieldPathValue("metadata.name", "c"),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := sortExtrasByFieldPath(tc.args.extras, tc.args.path)
			if diff := cmp.Diff(tc.want.err, got, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\n(...): -want err, +got err:\n%s", tc.reason, diff)
			}
			if tc.want.err != nil {
				return
			}
			if diff := cmp.Diff(tc.want.extras, tc.args.extras); diff != "" {
				t.Errorf("%s\n(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
