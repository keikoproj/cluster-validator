/*
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

package client

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

var (
	testBasePath = "test-files"
	NamespaceGVR = schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	NodeGVR      = schema.GroupVersionResource{Version: "v1", Resource: "nodes"}
	PodGVR       = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	DogGVR       = schema.GroupVersionResource{Group: "animals.io", Version: "v1alpha1", Resource: "dogs"}
)

func _fakeDynamicClient() *fake.FakeDynamicClient {
	return fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		NamespaceGVR: "NamespaceList",
		NodeGVR:      "NodeList",
		PodGVR:       "PodList",
		DogGVR:       "DogList",
	})
}

func _mockValidator(file string, cl *fake.FakeDynamicClient) *Validator {
	testPath := filepath.Join(testBasePath, file)
	spec, err := ParseValidationSpec(testPath)
	if err != nil {
		panic(err)
	}
	return NewValidator(cl, spec)
}

func _mockNamespace(cl *fake.FakeDynamicClient, name string, active bool) {
	var phase corev1.NamespacePhase
	if active {
		phase = corev1.NamespaceActive
	} else {
		phase = corev1.NamespaceTerminating
	}

	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NamespaceStatus{
			Phase: phase,
		},
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ns)
	if err != nil {
		panic(err)
	}

	unstructuredObj := &unstructured.Unstructured{
		Object: obj,
	}

	_, err = cl.Resource(NamespaceGVR).Create(context.Background(), unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func _mockNode(cl *fake.FakeDynamicClient, name string, ready bool) {
	var condition corev1.ConditionStatus
	if ready {
		condition = corev1.ConditionTrue
	} else {
		condition = corev1.ConditionFalse
	}

	ns := &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: condition,
				},
				{
					Type:   corev1.NodeMemoryPressure,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ns)
	if err != nil {
		panic(err)
	}

	unstructuredObj := &unstructured.Unstructured{
		Object: obj,
	}

	_, err = cl.Resource(NodeGVR).Create(context.Background(), unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func _mockPod(cl *fake.FakeDynamicClient, name, namespace string, running bool) {
	var phase corev1.PodPhase
	if running {
		phase = corev1.PodRunning
	} else {
		phase = corev1.PodPending
	}

	ns := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: phase,
		},
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ns)
	if err != nil {
		panic(err)
	}

	unstructuredObj := &unstructured.Unstructured{
		Object: obj,
	}

	_, err = cl.Resource(PodGVR).Namespace(namespace).Create(context.Background(), unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func _mockDog(cl *fake.FakeDynamicClient, name, namespace, phase string) {
	ns := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Dog",
			APIVersion: "animals.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ns)
	if err != nil {
		panic(err)
	}

	if err := unstructured.SetNestedField(obj, phase, "status", "phase"); err != nil {
		panic(err)
	}

	unstructuredObj := &unstructured.Unstructured{
		Object: obj,
	}

	_, err = cl.Resource(DogGVR).Namespace(namespace).Create(context.Background(), unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func Test_PositiveFieldValidation(t *testing.T) {
	g := gomega.NewWithT(t)
	gomega.RegisterTestingT(t)
	dynamic := _fakeDynamicClient()
	v := _mockValidator("field_validation.yaml", dynamic)
	_mockNamespace(dynamic, "test-namespace-1", true)
	_mockNamespace(dynamic, "test-namespace-2", true)
	_mockNamespace(dynamic, "other-namespace-3", false)
	validateErr := v.Validate()
	g.Expect(validateErr.Status).To(gomega.Equal(ValidationSucceeded))
}

func Test_NegativeFieldValidation(t *testing.T) {
	g := gomega.NewWithT(t)
	gomega.RegisterTestingT(t)
	dynamic := _fakeDynamicClient()
	v := _mockValidator("field_validation.yaml", dynamic)
	_mockNamespace(dynamic, "test-namespace-1", true)
	_mockNamespace(dynamic, "test-namespace-2", false)
	_mockNamespace(dynamic, "other-namespace-3", true)
	validateErr := v.Validate()
	g.Expect(validateErr.Status).To(gomega.Equal(ValidationFailed))
}

func Test_PositiveConditionValidation(t *testing.T) {
	g := gomega.NewWithT(t)
	gomega.RegisterTestingT(t)
	dynamic := _fakeDynamicClient()
	v := _mockValidator("condition_validation.yaml", dynamic)
	_mockNode(dynamic, "test-node-1", false)
	_mockNode(dynamic, "test-node-2", true)
	_mockNode(dynamic, "test-node-3", true)
	validateErr := v.Validate()
	g.Expect(validateErr.Status).To(gomega.Equal(ValidationSucceeded))
}

func Test_NegativeConditionValidation(t *testing.T) {
	g := gomega.NewWithT(t)
	gomega.RegisterTestingT(t)
	dynamic := _fakeDynamicClient()
	v := _mockValidator("condition_validation.yaml", dynamic)
	_mockNode(dynamic, "test-node-1", true)
	_mockNode(dynamic, "test-node-2", false)
	validateErr := v.Validate()
	g.Expect(validateErr.Status).To(gomega.Equal(ValidationFailed))
}

func Test_PositiveScopeValidation(t *testing.T) {
	g := gomega.NewWithT(t)
	gomega.RegisterTestingT(t)
	dynamic := _fakeDynamicClient()
	v := _mockValidator("scope_validation.yaml", dynamic)
	_mockPod(dynamic, "test-pod-1", "test-namespace-1", true)
	_mockPod(dynamic, "test-pod-2", "test-namespace-2", false)
	_mockPod(dynamic, "test-pod-3", "test-namespace-3", false)
	validateErr := v.Validate()
	g.Expect(validateErr.Status).To(gomega.Equal(ValidationSucceeded))
}

func Test_NegativeScopeValidation(t *testing.T) {
	g := gomega.NewWithT(t)
	gomega.RegisterTestingT(t)
	dynamic := _fakeDynamicClient()
	v := _mockValidator("scope_validation.yaml", dynamic)
	_mockPod(dynamic, "test-pod-1", "test-namespace-1", false)
	_mockPod(dynamic, "test-pod-2", "test-namespace-2", true)
	_mockPod(dynamic, "test-pod-3", "test-namespace-3", true)
	validateErr := v.Validate()
	g.Expect(validateErr.Status).To(gomega.Equal(ValidationFailed))
}

func Test_PositiveCustomValidation(t *testing.T) {
	g := gomega.NewWithT(t)
	gomega.RegisterTestingT(t)
	dynamic := _fakeDynamicClient()
	v := _mockValidator("custom_validation.yaml", dynamic)
	_mockDog(dynamic, "test-dog-1", "test-namespace-1", "woof")
	_mockDog(dynamic, "test-dog-2", "test-namespace-2", "woof")
	_mockDog(dynamic, "dog-3", "test-namespace-3", "bla")
	validateErr := v.Validate()
	g.Expect(validateErr.Status).To(gomega.Equal(ValidationSucceeded))
}

func Test_NegativeCustomValidation(t *testing.T) {
	g := gomega.NewWithT(t)
	gomega.RegisterTestingT(t)
	dynamic := _fakeDynamicClient()
	v := _mockValidator("custom_validation.yaml", dynamic)
	_mockDog(dynamic, "test-dog-1", "test-namespace-1", "woof")
	_mockDog(dynamic, "test-dog-2", "test-namespace-2", "bla")
	_mockDog(dynamic, "dog-3", "test-namespace-3", "bla")
	validateErr := v.Validate()
	g.Expect(validateErr.Status).To(gomega.Equal(ValidationFailed))
}

func Test_PositiveRequiredValidation(t *testing.T) {
	g := gomega.NewWithT(t)
	gomega.RegisterTestingT(t)
	dynamic := _fakeDynamicClient()
	v := _mockValidator("custom_validation.yaml", dynamic)
	_mockNamespace(dynamic, "test-namespace-1", false)
	_mockDog(dynamic, "test-dog-1", "test-namespace-1", "woof")
	_mockDog(dynamic, "test-dog-2", "test-namespace-2", "woof")
	_mockDog(dynamic, "dog-3", "test-namespace-3", "bla")
	validateErr := v.Validate()
	g.Expect(validateErr.Status).To(gomega.Equal(ValidationSucceeded))
}

func Test_ConfigurationOverride(t *testing.T) {
	g := gomega.NewWithT(t)
	gomega.RegisterTestingT(t)
	dynamic := _fakeDynamicClient()
	start := time.Now()
	v := _mockValidator("configuration_override.yaml", dynamic)
	_mockNamespace(dynamic, "test-namespace-1", true)
	validateErr := v.Validate()
	g.Expect(validateErr.Status).To(gomega.Equal(ValidationSucceeded))
	end := time.Now()
	elapsed := end.Sub(start)
	g.Expect(elapsed.Seconds()).To(gomega.BeNumerically(">", 0.45))
	g.Expect(elapsed.Seconds()).To(gomega.BeNumerically("<", 0.5))

}
