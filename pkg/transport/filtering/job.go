/*
Copyright 2024 The KubeStellar Authors.

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

package filtering

import (
	"github.com/kubestellar/kubestellar/pkg/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

func cleanJob(object *unstructured.Unstructured) {
	objectU := object.UnstructuredContent()
	mfes := object.GetManagedFields()
	fm := util.NewFieldMap("", mfes...)
	klog.InfoS("ManagedFields", "objName", object.GetName(), "mfes", mfes, "fm", fm)
	spec, found, _ := unstructured.NestedMap(objectU, "spec")
	if !found {
		klog.V(4).InfoS("No spec", "objName", object.GetName())
		return
	}
	fmSpec := fm.Advance(fieldpath.MakePathOrDie("spec"))
	selector, foundSelector, _ := unstructured.NestedMap(spec, "selector")
	if !foundSelector {
		klog.V(4).InfoS("No selector", "objName", object.GetName())
	} else {
		fmSelector := fmSpec.Advance(fieldpath.MakePathOrDie("selector"))
		trimMapByFieldMap(selector, fmSelector)
	}
	podLabels, foundlabels, _ := unstructured.NestedMap(spec, "template", "metadata", "labels")
	if !foundlabels {
		klog.V(4).InfoS("No pod labels", "objName", object.GetName())
	} else {
		fmPodLabels := fmSpec.Advance(fieldpath.MakePathOrDie("template", "metadata", "labels"))
		trimMapByFieldMap(podLabels, fmPodLabels)
	}
	_ = unstructured.SetNestedMap(objectU, podLabels, "spec", "template", "metadata", "labels")
	object.SetUnstructuredContent(objectU)
	klog.V(4).InfoS("Cleaned", "objectU", objectU, "fieldMap", fm, "foundSelector", foundSelector, "foundLabels", foundlabels)
}

func trimMapByFieldMap(obj map[string]any, fm util.FieldMap) {
	for key, val := range obj {
		fmVal := fm.Advance(fieldpath.MakePathOrDie(key))
		if fmVal.Empty() {
			delete(obj, key)
			continue
		}
		switch typed := val.(type) {
		case map[string]any:
			trimMapByFieldMap(typed, fmVal)
		case []any:
			trimSliceByFieldMap(typed, fmVal)
		}
	}
}

func trimSliceByFieldMap(obj []any, fm util.FieldMap) []any {
	ans := make([]any, 0, len(obj))
	for idx, val := range obj {
		fmVal := fm.Advance(fieldpath.MakePathOrDie(idx))
		if fmVal.Empty() {
			continue
		}
		outVal := val
		switch typed := outVal.(type) {
		case map[string]any:
			trimMapByFieldMap(typed, fmVal)
		case []any:
			outVal = trimSliceByFieldMap(typed, fmVal)
		}
		ans = append(ans, outVal)
	}
	return ans
}
