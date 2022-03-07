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
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/keikoproj/cluster-validator/pkg/api/v1alpha1"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	successEmoji = emoji.Sprint(":check_mark_button:")
	failEmoji    = emoji.Sprint(":fire:")
)

func (v *Validator) Validate() error {
	var (
		finished bool
		objs     = v.GetValidationObjects()
	)

	for _, obj := range objs {
		v.Waiter.Add(1)

		switch r := obj.(type) {
		case v1alpha1.ClusterResource:
			go v.validateClusterResource(r)
		case v1alpha1.ClusterEndpoint:
			go v.validateClusterEndpoint(r)
		case v1alpha1.HTTPEndpoint:
			//TODO
			continue

		}
	}

	go func() {
		v.Waiter.Wait()
		close(v.Waiter.finished)
	}()

	for {
		if finished {
			break
		}
		select {
		case <-v.Waiter.finished:
			finished = true
		case err := <-v.Waiter.errors:
			return err
		}
	}

	return nil
}

func (v *Validator) validateClusterResource(r v1alpha1.ClusterResource) {
	defer v.Waiter.Done()
	var (
		summary                    = ValidationSummary{}
		resourceName               = r.Name
		successCount, failureCount int
		globalCfg                  = v.GetGlobalConfiguration()
		successThreshold           = r.SuccessThreshold(globalCfg)
		failureThreshold           = r.FailureThreshold(globalCfg)
	)
	log.Infof("validating resource '%v'", resourceName)

	for {
		err := v.listDynamicResource(r)
		if err != nil {
			v.Waiter.errors <- err
		}

		resources := v.getValidationResources(r)

		if summary, err = v.validateResources(r, resources); err != nil {
			failureCount++
			successCount = 0
			log.Warnf("validation of '%v' failed (%v/%v) -> %v", resourceName, failureCount, failureThreshold, err)
		} else {
			successCount++
			failureCount = 0
			log.Infof("validation of '%v' successful (%v/%v)", resourceName, successCount, successThreshold)
		}

		if successCount >= successThreshold {
			if !reflect.DeepEqual(summary, ValidationSummary{}) {
				prettyPrintStruct(summary)
			}
			log.Infof("%v resource '%v' validated successfully", successEmoji, resourceName)
			return
		} else if failureCount >= failureThreshold {
			if !reflect.DeepEqual(summary, ValidationSummary{}) {
				prettyPrintStruct(summary)
			}
			if r.Required {
				v.Waiter.errors <- ValidationError{
					Message:              errors.Errorf("failure threshold met for resource '%v'", resourceName),
					GVR:                  groupVersionResource(r.APIVersion, r.Name),
					FieldValidations:     summary.FieldValidation,
					ConditionValidations: summary.ConditionValidation,
				}
			}
			log.Warnf("%v resource '%v' validation failed", failEmoji, resourceName)
			return
		}
		time.Sleep(r.Interval(globalCfg))
	}
}

func (v *Validator) validateClusterEndpoint(r v1alpha1.ClusterEndpoint) {
	defer v.Waiter.Done()

	var (
		summary                    = ValidationSummary{}
		resourceName               = r.Name
		successCount, failureCount int
		globalCfg                  = v.GetGlobalConfiguration()
		successThreshold           = r.SuccessThreshold(globalCfg)
		failureThreshold           = r.FailureThreshold(globalCfg)
	)

	log.Infof("validating cluster endpoint '%v'", resourceName)

	for {
		res := NewClusterEndpointValidationResult(r.Name)

		if out, err := rawGet(v.RESTClient, r.URI); err != nil {
			failureCount++
			successCount = 0
			res.Errors[r.URI] = err.Error()
			log.Warnf("validation of cluster endpoint '%v' failed (%v/%v) -> %v", resourceName, failureCount, failureThreshold, err)
		} else {
			successCount++
			failureCount = 0
			log.Debugf("rawGet output for %v: %v", r.Name, out.String())
			log.Infof("validation of '%v' successful (%v/%v)", resourceName, successCount, successThreshold)
		}

		if successCount >= successThreshold {
			if !reflect.DeepEqual(summary, ValidationSummary{}) {
				prettyPrintStruct(summary)
			}
			log.Infof("%v resource '%v' validated successfully", successEmoji, resourceName)
			return
		} else if failureCount >= failureThreshold {
			summary.ClusterEndpointValidation = append(summary.ClusterEndpointValidation, res)
			if !reflect.DeepEqual(summary, ValidationSummary{}) {
				prettyPrintStruct(summary)
			}
			if r.Required {
				v.Waiter.errors <- ValidationError{
					Message:                    errors.Errorf("failure threshold met for resource '%v'", resourceName),
					ClusterEndpointValidations: summary.ClusterEndpointValidation,
				}
			}
			log.Warnf("%v resource '%v' validation failed", failEmoji, resourceName)
			return
		}
		time.Sleep(r.Interval(globalCfg))
	}
}

func (v *Validator) getValidationResources(resource v1alpha1.ClusterResource) []unstructured.Unstructured {

	var (
		validationResources = make([]unstructured.Unstructured, 0)
	)

	v.RLock()
	for _, r := range v.ClusterResources[resource.Name] {

		var (
			namespace = r.GetNamespace()
			name      = r.GetName()
		)

		if !inSelectionScope(resource.Namespaces, namespace) {
			continue
		}

		if !inSelectionScope(resource.Names, name) {
			continue
		}

		validationResources = append(validationResources, r)
	}
	v.RUnlock()

	return validationResources
}

func (v *Validator) validateResources(r v1alpha1.ClusterResource, resources []unstructured.Unstructured) (ValidationSummary, error) {

	var (
		summary = ValidationSummary{}
		failed  bool
	)

	fields := v.validateFields(r, resources)
	if len(fields) > 0 {
		summary.FieldValidation = fields
		failed = true
	}

	conditions := v.validateConditions(r, resources)
	if len(conditions) > 0 {
		summary.ConditionValidation = conditions
		failed = true
	}

	if failed {
		return summary, errors.New("failed to validate resources")
	}

	return summary, nil
}

func (v *Validator) validateConditions(r v1alpha1.ClusterResource, resources []unstructured.Unstructured) []ConditionValidationResult {
	var (
		failedValidations = make([]ConditionValidationResult, 0)
	)

	for _, cond := range r.Conditions {

		var (
			conditionStatus = cond.Status
			conditionType   = cond.Type
			JSONPath        = cond.Path
			conditionStr    = fmt.Sprintf("%v=%v", conditionType, conditionStatus)
			result          = NewConditionValidationResult(conditionStr)
		)

		for _, resource := range resources {
			var (
				name           = namespacedName(resource)
				conditionMatch bool
			)

			conditions, ok, err := unstructuredSlicePath(resource, JSONPath)
			if err != nil {
				reason := fmt.Sprintf("type mismatch in path %v: %v", JSONPath, err)
				result.ResourceErrors[reason] = append(result.ResourceErrors[reason], name)
			}

			if !ok {
				reason := fmt.Sprintf("conditions not found in resource path %v", JSONPath)
				result.ResourceErrors[reason] = append(result.ResourceErrors[reason], name)
			}

			for _, c := range conditions {
				condition, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				tp, found := condition["type"]
				if !found {
					continue
				}
				condType, ok := tp.(string)
				if !ok {
					continue
				}
				if strings.EqualFold(condType, conditionType) {
					status := condition["status"].(string)
					conditionMatch = true
					if !strings.EqualFold(status, string(conditionStatus)) {
						reason := fmt.Sprintf("found conditions status '%v' does not match required status '%v'", status, conditionStatus)
						result.ResourceErrors[reason] = append(result.ResourceErrors[reason], name)
					}
				}
			}

			if !conditionMatch {
				reason := fmt.Sprintf("condition type '%v' was not found in resource path %v", conditionType, JSONPath)
				result.ResourceErrors[reason] = append(result.ResourceErrors[reason], name)
			}
		}

		if len(result.ResourceErrors) > 0 {
			failedValidations = append(failedValidations, result)
		}
	}

	return failedValidations
}

func (v *Validator) validateFields(r v1alpha1.ClusterResource, resources []unstructured.Unstructured) []FieldValidationResult {
	var (
		failedValidations = make([]FieldValidationResult, 0)
	)

	for _, field := range r.Fields {

		var (
			JSONPath   = field.GetPath()
			pathValues = field.GetValues()
			result     = NewFieldValidationResult(field.Path)
		)

		for _, resource := range resources {
			var name = namespacedName(resource)

			val, err := getJsonPathValue(resource, JSONPath)
			if err != nil {
				reason := fmt.Sprintf("field '%v' has type mismatch: %v", field.Path, err)
				result.ResourceErrors[reason] = append(result.ResourceErrors[reason], name)
			}

			if !matchInPatterns(pathValues, val) {
				reason := fmt.Sprintf("JSONPath values '%v' not matching '%v' in resources", pathValues, val)
				result.ResourceErrors[reason] = append(result.ResourceErrors[reason], name)
			}
		}

		if len(result.ResourceErrors) > 0 {
			failedValidations = append(failedValidations, result)
		}
	}

	return failedValidations
}

func (v *Validator) listDynamicResource(resource v1alpha1.ClusterResource) error {
	var (
		gvr = groupVersionResource(resource.APIVersion, resource.Name)
	)

	resources, err := v.Kubernetes.Resource(gvr).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to list dynamic resource '%v'", gvr)
	}
	v.Lock()
	v.ClusterResources[resource.Name] = resources.Items
	v.Unlock()
	return nil
}
