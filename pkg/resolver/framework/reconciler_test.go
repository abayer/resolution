/*
Copyright 2022 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/tektoncd/resolution/pkg/apis/resolution/v1alpha1"
	resolutioncommon "github.com/tektoncd/resolution/pkg/common"
	"github.com/tektoncd/resolution/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name              string
		inputRequest      *v1alpha1.ResolutionRequest
		paramMap          map[string]*FakeResolvedResource
		reconcilerTimeout time.Duration
		expectedStatus    *v1alpha1.ResolutionRequestStatus
		expectedErr       error
	}{
		{
			name: "unknown value",
			inputRequest: &v1alpha1.ResolutionRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "resolution.tekton.dev/v1alpha1",
					Kind:       "ResolutionRequest",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rr",
					Namespace:         "foo",
					CreationTimestamp: metav1.Time{Time: time.Now()},
					Labels: map[string]string{
						resolutioncommon.LabelKeyResolverType: LabelValueFakeResolverType,
					},
				},
				Spec: v1alpha1.ResolutionRequestSpec{
					Parameters: map[string]string{
						FakeParamName: "bar",
					},
				},
				Status: v1alpha1.ResolutionRequestStatus{},
			},
			expectedErr: errors.New("error getting \"Fake\" \"foo/rr\": couldn't find resource for param value bar"),
		}, {
			name: "known value",
			inputRequest: &v1alpha1.ResolutionRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "resolution.tekton.dev/v1alpha1",
					Kind:       "ResolutionRequest",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rr",
					Namespace:         "foo",
					CreationTimestamp: metav1.Time{Time: time.Now()},
					Labels: map[string]string{
						resolutioncommon.LabelKeyResolverType: LabelValueFakeResolverType,
					},
				},
				Spec: v1alpha1.ResolutionRequestSpec{
					Parameters: map[string]string{
						FakeParamName: "bar",
					},
				},
				Status: v1alpha1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*FakeResolvedResource{
				"bar": {
					Content:       "some content",
					AnnotationMap: map[string]string{"foo": "bar"},
				},
			},
			expectedStatus: &v1alpha1.ResolutionRequestStatus{
				Status: duckv1.Status{
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
				ResolutionRequestStatusFields: v1alpha1.ResolutionRequestStatusFields{
					Data: base64.StdEncoding.Strict().EncodeToString([]byte("some content")),
				},
			},
		}, {
			name: "error resolving",
			inputRequest: &v1alpha1.ResolutionRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "resolution.tekton.dev/v1alpha1",
					Kind:       "ResolutionRequest",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rr",
					Namespace:         "foo",
					CreationTimestamp: metav1.Time{Time: time.Now()},
					Labels: map[string]string{
						resolutioncommon.LabelKeyResolverType: LabelValueFakeResolverType,
					},
				},
				Spec: v1alpha1.ResolutionRequestSpec{
					Parameters: map[string]string{
						FakeParamName: "bar",
					},
				},
				Status: v1alpha1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*FakeResolvedResource{
				"bar": {
					ErrorWith: "fake failure",
				},
			},
			expectedErr: errors.New(`error getting "Fake" "foo/rr": fake failure`),
		}, {
			name: "timeout",
			inputRequest: &v1alpha1.ResolutionRequest{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "resolution.tekton.dev/v1alpha1",
					Kind:       "ResolutionRequest",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rr",
					Namespace:         "foo",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-59 * time.Second)}, // 1 second before default timeout
					Labels: map[string]string{
						resolutioncommon.LabelKeyResolverType: LabelValueFakeResolverType,
					},
				},
				Spec: v1alpha1.ResolutionRequestSpec{
					Parameters: map[string]string{
						FakeParamName: "bar",
					},
				},
				Status: v1alpha1.ResolutionRequestStatus{},
			},
			paramMap: map[string]*FakeResolvedResource{
				"bar": {
					WaitFor: 1100 * time.Millisecond,
				},
			},
			reconcilerTimeout: 1 * time.Second,
			expectedErr:       errors.New("context deadline exceeded"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				ResolutionRequests: []*v1alpha1.ResolutionRequest{tc.inputRequest},
			}

			fakeResolver := &FakeResolver{ForParam: tc.paramMap}
			if tc.reconcilerTimeout > 0 {
				fakeResolver.Timeout = tc.reconcilerTimeout
			}

			RunResolverReconcileTest(t, d, fakeResolver, tc.inputRequest, tc.expectedStatus, tc.expectedErr)
		})
	}
}
