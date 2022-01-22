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
	successEmoji                         = emoji.Sprint(":check_mark_button:")
	failEmoji                            = emoji.Sprint(":fire:")
	ValidationFailed    ValidationStatus = "failed"
	ValidationSucceeded ValidationStatus = "succeeded"
)

func (v *Validator) Validate() ValidationError {
	var (
		finished bool
	)
	for _, resource := range v.GetResources() {
		v.Waiter.Add(1)
		var (
			resourceName = resource.Name
		)

		go func(r v1alpha1.ClusterResource) {
			defer v.Waiter.Done()

			var (
				successCount, failureCount int
				successThreshold           = v.successThreshold(r)
				failureThreshold           = v.failureThreshold(r)
				summary                    ValidationSummary
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
						v.Waiter.summary <- summary
						prettyPrintStruct(summary)
					}
					log.Infof("%v resource '%v' validated successfully", successEmoji, resourceName)
					return
				} else if failureCount >= failureThreshold {
					if !reflect.DeepEqual(summary, ValidationSummary{}) {
						v.Waiter.summary <- summary
						prettyPrintStruct(summary)
					}
					if r.Required {
						v.Waiter.errors <- errors.Errorf("failure threshold met for resource '%v'", resourceName)
					}
					log.Warnf("%v resource '%v' validation failed", failEmoji, resourceName)
					return
				}
				time.Sleep(v.interval(r))
			}

		}(resource)
	}

	go func() {
		v.Waiter.Wait()
		close(v.Waiter.finished)
	}()

	summaries := []ValidationSummary{}
	for {
		if finished {
			break
		}
		select {
		case <-v.Waiter.finished:
			finished = true
		case summary := <-v.Waiter.summary:
			summaries = append(summaries, summary)
		case err := <-v.Waiter.errors:
			return ValidationError{
				Status:    ValidationFailed,
				Message:   err,
				Summaries: summaries,
			}
		}
	}

	return ValidationError{
		Status:    ValidationSucceeded,
		Message:   errors.Errorf("test"),
		Summaries: summaries,
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
		summary.GVR = groupVersionResource(r.APIVersion, r.Name)
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
			JSONPath   = field.JSONPath()
			pathValues = field.Values()
			result     = NewFieldValidationResult(field.Path)
		)

		for _, resource := range resources {
			var name = namespacedName(resource)

			val, ok, err := unstructuredPath(resource, JSONPath)
			if err != nil {
				reason := fmt.Sprintf("field '%v' has type mismatch: %v", field.Path, err)
				result.ResourceErrors[reason] = append(result.ResourceErrors[reason], name)
			}

			if !ok {
				reason := fmt.Sprintf("could not find JSONPath '%v' in resources", field.Path)
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

func (v *Validator) successThreshold(resource v1alpha1.ClusterResource) int {
	var (
		resourceCfg = resource.GetConfiguration()
		globalCfg   = v.Validation.GetConfiguration()
	)

	if resourceCfg.SuccessThreshold > 0 {
		return resourceCfg.SuccessThreshold
	} else {
		return globalCfg.SuccessThreshold
	}
}

func (v *Validator) failureThreshold(resource v1alpha1.ClusterResource) int {
	var (
		resourceCfg = resource.GetConfiguration()
		globalCfg   = v.Validation.GetConfiguration()
	)

	if resourceCfg.FailureThreshold > 0 {
		return resourceCfg.FailureThreshold
	} else {
		return globalCfg.FailureThreshold
	}
}

func (v *Validator) interval(resource v1alpha1.ClusterResource) time.Duration {
	var (
		resourceCfg = resource.GetConfiguration()
		globalCfg   = v.Validation.GetConfiguration()
	)

	if resourceCfg.Interval != "" {
		d, err := time.ParseDuration(resourceCfg.Interval)
		if err != nil {
			log.Warnf("failed to parse duration '%v', using default of 1s", resourceCfg.Interval)
			return time.Second * 1
		}
		return d
	} else {
		d, err := time.ParseDuration(globalCfg.Interval)
		if err != nil {
			log.Warnf("failed to parse duration '%v', using default of 1s", globalCfg.Interval)
			return time.Second * 1
		}
		return d
	}
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
