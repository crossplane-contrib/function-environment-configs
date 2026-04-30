package main

import (
	"encoding/json"
	"testing"

	"github.com/crossplane-contrib/function-environment-configs/input/v1beta1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
)

func TestMapResolve(t *testing.T) {
	asJSON := func(val any) extv1.JSON {
		raw, err := json.Marshal(val)
		if err != nil {
			t.Fatal(err)
		}
		res := extv1.JSON{}
		if err := json.Unmarshal(raw, &res); err != nil {
			t.Fatal(err)
		}
		return res
	}

	type args struct {
		t *v1beta1.MapTransform
		i any
	}
	type want struct {
		o   any
		err error
	}

	cases := map[string]struct {
		args
		want
	}{
		"NonStringInput": {
			args: args{
				t: &v1beta1.MapTransform{},
				i: 5,
			},
			want: want{
				err: errors.Errorf(errFmtMapTypeNotSupported, "int"),
			},
		},
		"KeyNotFound": {
			args: args{
				t: &v1beta1.MapTransform{},
				i: "ola",
			},
			want: want{
				err: errors.Errorf(errFmtMapNotFound, "ola"),
			},
		},
		"SuccessString": {
			args: args{
				t: &v1beta1.MapTransform{Pairs: map[string]extv1.JSON{"ola": asJSON("voila")}},
				i: "ola",
			},
			want: want{
				o: "voila",
			},
		},
		"SuccessNumber": {
			args: args{
				t: &v1beta1.MapTransform{Pairs: map[string]extv1.JSON{"ola": asJSON(1.0)}},
				i: "ola",
			},
			want: want{
				o: 1.0,
			},
		},
		"SuccessBoolean": {
			args: args{
				t: &v1beta1.MapTransform{Pairs: map[string]extv1.JSON{"ola": asJSON(true)}},
				i: "ola",
			},
			want: want{
				o: true,
			},
		},
		"SuccessObject": {
			args: args{
				t: &v1beta1.MapTransform{Pairs: map[string]extv1.JSON{"ola": asJSON(map[string]any{"foo": "bar"})}},
				i: "ola",
			},
			want: want{
				o: map[string]any{"foo": "bar"},
			},
		},
		"SuccessSlice": {
			args: args{
				t: &v1beta1.MapTransform{Pairs: map[string]extv1.JSON{"ola": asJSON([]string{"foo", "bar"})}},
				i: "ola",
			},
			want: want{
				o: []any{"foo", "bar"},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ResolveMap(tc.t, tc.i)

			if diff := cmp.Diff(tc.o, got); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestMatchResolve(t *testing.T) {
	asJSON := func(val any) extv1.JSON {
		raw, err := json.Marshal(val)
		if err != nil {
			t.Fatal(err)
		}
		res := extv1.JSON{}
		if err := json.Unmarshal(raw, &res); err != nil {
			t.Fatal(err)
		}
		return res
	}

	type args struct {
		t *v1beta1.MatchTransform
		i any
	}
	type want struct {
		o   any
		err error
	}

	cases := map[string]struct {
		args
		want
	}{
		"ErrNonStringInput": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns: []v1beta1.MatchTransformPattern{
						{
							Type:    v1beta1.MatchTransformPatternTypeLiteral,
							Literal: ptr.To[string]("5"),
						},
					},
				},
				i: 5,
			},
			want: want{
				err: errors.Wrapf(errors.Errorf(errFmtMatchInputTypeInvalid, "int"), errFmtMatchPattern, 0),
			},
		},
		"ErrFallbackValueAndToInput": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns:      []v1beta1.MatchTransformPattern{},
					FallbackValue: asJSON("foo"),
					FallbackTo:    "Input",
				},
				i: "foo",
			},
			want: want{
				err: errors.New(errMatchFallbackBoth),
			},
		},
		"NoPatternsFallback": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns:      []v1beta1.MatchTransformPattern{},
					FallbackValue: asJSON("bar"),
				},
				i: "foo",
			},
			want: want{
				o: "bar",
			},
		},
		"NoPatternsFallbackToValueExplicit": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns:      []v1beta1.MatchTransformPattern{},
					FallbackValue: asJSON("bar"),
					FallbackTo:    "Value", // Explicitly set to Value, unnecessary but valid.
				},
				i: "foo",
			},
			want: want{
				o: "bar",
			},
		},
		"NoPatternsFallbackNil": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns:      []v1beta1.MatchTransformPattern{},
					FallbackValue: asJSON(nil),
				},
				i: "foo",
			},
			want: want{},
		},
		"NoPatternsFallbackToInput": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns:   []v1beta1.MatchTransformPattern{},
					FallbackTo: "Input",
				},
				i: "foo",
			},
			want: want{
				o: "foo",
			},
		},
		"NoPatternsFallbackNilToInput": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns:      []v1beta1.MatchTransformPattern{},
					FallbackValue: asJSON(nil),
					FallbackTo:    "Input",
				},
				i: "foo",
			},
			want: want{
				o: "foo",
			},
		},
		"MatchLiteral": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns: []v1beta1.MatchTransformPattern{
						{
							Type:    v1beta1.MatchTransformPatternTypeLiteral,
							Literal: ptr.To[string]("foo"),
							Result:  asJSON("bar"),
						},
					},
				},
				i: "foo",
			},
			want: want{
				o: "bar",
			},
		},
		"MatchLiteralFirst": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns: []v1beta1.MatchTransformPattern{
						{
							Type:    v1beta1.MatchTransformPatternTypeLiteral,
							Literal: ptr.To[string]("foo"),
							Result:  asJSON("bar"),
						},
						{
							Type:    v1beta1.MatchTransformPatternTypeLiteral,
							Literal: ptr.To[string]("foo"),
							Result:  asJSON("not this"),
						},
					},
				},
				i: "foo",
			},
			want: want{
				o: "bar",
			},
		},
		"MatchLiteralWithResultStruct": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns: []v1beta1.MatchTransformPattern{
						{
							Type:    v1beta1.MatchTransformPatternTypeLiteral,
							Literal: ptr.To[string]("foo"),
							Result: asJSON(map[string]any{
								"Hello": "World",
							}),
						},
					},
				},
				i: "foo",
			},
			want: want{
				o: map[string]any{
					"Hello": "World",
				},
			},
		},
		"MatchLiteralWithResultSlice": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns: []v1beta1.MatchTransformPattern{
						{
							Type:    v1beta1.MatchTransformPatternTypeLiteral,
							Literal: ptr.To[string]("foo"),
							Result: asJSON([]string{
								"Hello", "World",
							}),
						},
					},
				},
				i: "foo",
			},
			want: want{
				o: []any{
					"Hello", "World",
				},
			},
		},
		"MatchLiteralWithResultNumber": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns: []v1beta1.MatchTransformPattern{
						{
							Type:    v1beta1.MatchTransformPatternTypeLiteral,
							Literal: ptr.To[string]("foo"),
							Result:  asJSON(5),
						},
					},
				},
				i: "foo",
			},
			want: want{
				o: 5.0,
			},
		},
		"MatchLiteralWithResultBool": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns: []v1beta1.MatchTransformPattern{
						{
							Type:    v1beta1.MatchTransformPatternTypeLiteral,
							Literal: ptr.To[string]("foo"),
							Result:  asJSON(true),
						},
					},
				},
				i: "foo",
			},
			want: want{
				o: true,
			},
		},
		"MatchLiteralWithResultNil": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns: []v1beta1.MatchTransformPattern{
						{
							Type:    v1beta1.MatchTransformPatternTypeLiteral,
							Literal: ptr.To[string]("foo"),
							Result:  asJSON(nil),
						},
					},
				},
				i: "foo",
			},
			want: want{},
		},
		"MatchRegexp": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns: []v1beta1.MatchTransformPattern{
						{
							Type:   v1beta1.MatchTransformPatternTypeRegexp,
							Regexp: ptr.To[string]("^foo.*$"),
							Result: asJSON("Hello World"),
						},
					},
				},
				i: "foobar",
			},
			want: want{
				o: "Hello World",
			},
		},
		"ErrMissingRegexp": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns: []v1beta1.MatchTransformPattern{
						{
							Type: v1beta1.MatchTransformPatternTypeRegexp,
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.Errorf(errFmtRequiredField, "regexp", string(v1beta1.MatchTransformPatternTypeRegexp)), errFmtMatchPattern, 0),
			},
		},
		"ErrInvalidRegexp": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns: []v1beta1.MatchTransformPattern{
						{
							Type:   v1beta1.MatchTransformPatternTypeRegexp,
							Regexp: ptr.To[string]("?="),
						},
					},
				},
			},
			want: want{
				// This might break if Go's regexp changes its internal error
				// messages:
				err: errors.Wrapf(errors.Wrapf(errors.Wrap(errors.Wrap(errors.New("`?`"), "missing argument to repetition operator"), "error parsing regexp"), errMatchRegexpCompile), errFmtMatchPattern, 0),
			},
		},
		"ErrMissingLiteral": {
			args: args{
				t: &v1beta1.MatchTransform{
					Patterns: []v1beta1.MatchTransformPattern{
						{
							Type: v1beta1.MatchTransformPatternTypeLiteral,
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.Errorf(errFmtRequiredField, "literal", string(v1beta1.MatchTransformPatternTypeLiteral)), errFmtMatchPattern, 0),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ResolveMatch(tc.t, tc.i)

			if diff := cmp.Diff(tc.o, got); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestStringResolve(t *testing.T) {
	type args struct {
		stype   v1beta1.StringTransformType
		fmts    *string
		convert *v1beta1.StringConversionType
		trim    *string
		regexp  *v1beta1.StringTransformRegexp
		replace *v1beta1.StringTransformReplace
		i       any
	}
	type want struct {
		o   string
		err error
	}
	sFmt := "verycool%s"
	iFmt := "the largest %d"

	upper := v1beta1.StringConversionTypeToUpper
	lower := v1beta1.StringConversionTypeToLower
	wrongConvertType := v1beta1.StringConversionType("Something")

	prefix := "https://"
	suffix := "-test"

	cases := map[string]struct {
		args
		want
	}{
		"NotSupportedType": {
			args: args{
				stype: "Something",
				i:     "value",
			},
			want: want{
				err: errors.Errorf(errStringTransformTypeFailed, "Something"),
			},
		},
		"FmtFailed": {
			args: args{
				stype: v1beta1.StringTransformTypeFormat,
				i:     "value",
			},
			want: want{
				err: errors.Errorf(errStringTransformTypeFormat, string(v1beta1.StringTransformTypeFormat)),
			},
		},
		"FmtString": {
			args: args{
				stype: v1beta1.StringTransformTypeFormat,
				fmts:  &sFmt,
				i:     "thing",
			},
			want: want{
				o: "verycoolthing",
			},
		},
		"FmtInteger": {
			args: args{
				stype: v1beta1.StringTransformTypeFormat,
				fmts:  &iFmt,
				i:     8,
			},
			want: want{
				o: "the largest 8",
			},
		},
		"ConvertNotSet": {
			args: args{
				stype: v1beta1.StringTransformTypeConvert,
				i:     "crossplane",
			},
			want: want{
				err: errors.Errorf(errStringTransformTypeConvert, string(v1beta1.StringTransformTypeConvert)),
			},
		},
		"ConvertTypFailed": {
			args: args{
				stype:   v1beta1.StringTransformTypeConvert,
				convert: &wrongConvertType,
				i:       "crossplane",
			},
			want: want{
				err: errors.Errorf(errStringConvertTypeFailed, wrongConvertType),
			},
		},
		"ConvertToUpper": {
			args: args{
				stype:   v1beta1.StringTransformTypeConvert,
				convert: &upper,
				i:       "crossplane",
			},
			want: want{
				o: "CROSSPLANE",
			},
		},
		"ConvertToLower": {
			args: args{
				stype:   v1beta1.StringTransformTypeConvert,
				convert: &lower,
				i:       "CrossPlane",
			},
			want: want{
				o: "crossplane",
			},
		},
		"TrimPrefix": {
			args: args{
				stype: v1beta1.StringTransformTypeTrimPrefix,
				trim:  &prefix,
				i:     "https://crossplane.io",
			},
			want: want{
				o: "crossplane.io",
			},
		},
		"TrimSuffix": {
			args: args{
				stype: v1beta1.StringTransformTypeTrimSuffix,
				trim:  &suffix,
				i:     "my-string-test",
			},
			want: want{
				o: "my-string",
			},
		},
		"TrimPrefixWithoutMatch": {
			args: args{
				stype: v1beta1.StringTransformTypeTrimPrefix,
				trim:  &prefix,
				i:     "crossplane.io",
			},
			want: want{
				o: "crossplane.io",
			},
		},
		"TrimSuffixWithoutMatch": {
			args: args{
				stype: v1beta1.StringTransformTypeTrimSuffix,
				trim:  &suffix,
				i:     "my-string",
			},
			want: want{
				o: "my-string",
			},
		},
		"RegexpNotCompiling": {
			args: args{
				stype: v1beta1.StringTransformTypeRegexp,
				regexp: &v1beta1.StringTransformRegexp{
					Match: "[a-z",
				},
				i: "my-string",
			},
			want: want{
				err: errors.Wrap(errors.New("error parsing regexp: missing closing ]: `[a-z`"), errStringTransformTypeRegexpFailed),
			},
		},
		"RegexpSimpleMatch": {
			args: args{
				stype: v1beta1.StringTransformTypeRegexp,
				regexp: &v1beta1.StringTransformRegexp{
					Match: "[0-9]",
				},
				i: "my-1-string",
			},
			want: want{
				o: "1",
			},
		},
		"RegexpCaptureGroup": {
			args: args{
				stype: v1beta1.StringTransformTypeRegexp,
				regexp: &v1beta1.StringTransformRegexp{
					Match: "my-([0-9]+)-string",
					Group: ptr.To[int](1),
				},
				i: "my-1-string",
			},
			want: want{
				o: "1",
			},
		},
		"RegexpReplaceWithBackreferences": {
			args: args{
				stype: v1beta1.StringTransformTypeRegexp,
				regexp: &v1beta1.StringTransformRegexp{
					Match:   "^team-(.+)-(.+)$",
					Replace: ptr.To[string]("${1}-${2}-environment-config"),
				},
				i: "team-alpha-prod",
			},
			want: want{
				o: "alpha-prod-environment-config",
			},
		},
		"RegexpReplaceSwapGroups": {
			args: args{
				stype: v1beta1.StringTransformTypeRegexp,
				regexp: &v1beta1.StringTransformRegexp{
					Match:   "^team-(.+)-(.+)$",
					Replace: ptr.To[string]("${2}-${1}-environment-config"),
				},
				i: "team-alpha-prod",
			},
			want: want{
				o: "prod-alpha-environment-config",
			},
		},
		"RegexpReplaceNoMatch": {
			args: args{
				stype: v1beta1.StringTransformTypeRegexp,
				regexp: &v1beta1.StringTransformRegexp{
					Match:   "^team-(.+)-(.+)$",
					Replace: ptr.To[string]("${1}-environment-config"),
				},
				i: "no-match-here",
			},
			want: want{
				o: "no-match-here",
			},
		},
		"RegexpNoSuchCaptureGroup": {
			args: args{
				stype: v1beta1.StringTransformTypeRegexp,
				regexp: &v1beta1.StringTransformRegexp{
					Match: "my-([0-9]+)-string",
					Group: ptr.To[int](2),
				},
				i: "my-1-string",
			},
			want: want{
				err: errors.Errorf(errStringTransformTypeRegexpNoMatch, "my-([0-9]+)-string", 2),
			},
		},
		"ReplaceFound": {
			args: args{
				stype: v1beta1.StringTransformTypeReplace,
				replace: &v1beta1.StringTransformReplace{
					Search:  "Cr",
					Replace: "B",
				},
				i: "Crossplane",
			},
			want: want{
				o: "Bossplane",
			},
		},
		"ReplaceNotFound": {
			args: args{
				stype: v1beta1.StringTransformTypeReplace,
				replace: &v1beta1.StringTransformReplace{
					Search:  "xx",
					Replace: "zz",
				},
				i: "Crossplane",
			},
			want: want{
				o: "Crossplane",
			},
		},
		"ReplaceRemove": {
			args: args{
				stype: v1beta1.StringTransformTypeReplace,
				replace: &v1beta1.StringTransformReplace{
					Search:  "ss",
					Replace: "",
				},
				i: "Crossplane",
			},
			want: want{
				o: "Croplane",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tr := &v1beta1.StringTransform{
				Type:    tc.stype,
				Format:  tc.fmts,
				Convert: tc.convert,
				Trim:    tc.trim,
				Regexp:  tc.regexp,
				Replace: tc.replace,
			}

			got, err := ResolveString(tr, tc.i)

			if diff := cmp.Diff(tc.o, got); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
		})
	}
}
