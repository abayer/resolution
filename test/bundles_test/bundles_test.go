//go:build e2e

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

package bundles_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tektoncd/resolution/pkg/apis/resolution/v1alpha1"
	"github.com/tektoncd/resolution/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	knativetest "knative.dev/pkg/test"
)

// testNamespace is the namespace to construct and run this e2e test in.
const testNamespace = "tekton-resolution-bundles-test"

// waitInterval is the duration between repeat attempts to check on the
// status of the test's resolution request.
const waitInterval = time.Second

// waitTimeout is the total maximum time the test may spend waiting for
// successful resolution of the test's bundle request.
const waitTimeout = 20 * time.Second

// TestBundlesSmoke creates a resolution request for a bundle and checks
// that it succeeds.
func TestBundlesSmoke(t *testing.T) {
	ctx := context.Background()
	configPath := knativetest.Flags.Kubeconfig
	clusterName := knativetest.Flags.Cluster

	requestYAML, err := os.ReadFile("./resolution-request.yaml")
	if err != nil {
		t.Fatalf("unable to read resolution request yaml fixture: %v", err)
	}

	req := &v1alpha1.ResolutionRequest{}
	_, _, err = scheme.Codecs.UniversalDeserializer().Decode(requestYAML, nil, req)
	if err != nil {
		t.Fatalf("error parsing resolution request yaml fixture: %v", err)
	}

	cfg, err := knativetest.BuildClientConfig(configPath, clusterName)

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("failed to create kubeclient from config file at %s: %s", configPath, err)
	}

	tearDown := func() {
		err := kubeClient.CoreV1().Namespaces().Delete(ctx, testNamespace, metav1.DeleteOptions{})
		if err != nil {
			t.Errorf("error deleting test namespace %q: %v", testNamespace, err)
		}
	}

	knativetest.CleanupOnInterrupt(tearDown, t.Logf)
	defer tearDown()

	_, err = kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
			Labels: map[string]string{
				"resolution.tekton.dev/test-e2e": "true",
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create namespace %s for tests: %s", testNamespace, err)
	}

	clientset, err := versioned.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("error getting resolution clientset: %v", err)
	}

	_, err = clientset.ResolutionV1alpha1().ResolutionRequests(testNamespace).Create(ctx, req, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("error creating request: %v", err)
	}

	err = wait.PollImmediate(waitInterval, waitTimeout, func() (bool, error) {
		latestResolutionRequest, err := clientset.ResolutionV1alpha1().ResolutionRequests(testNamespace).Get(ctx, req.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		resolvedData := latestResolutionRequest.Status.ResolutionRequestStatusFields.Data
		if resolvedData != "" {
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		t.Fatalf("error waiting for completed resolution request: %v", err)
	}
}
