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

package util

import (
	"bytes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

type FieldMap []ManagedFieldMap

type ManagedFieldMap struct {
	MFE    metav1.ManagedFieldsEntry // but with FieldsV1 empty
	Fields *fieldpath.Set
}

func NewFieldMap(subresource string, mfes ...metav1.ManagedFieldsEntry) FieldMap {
	ans := []ManagedFieldMap{}
	for _, mfe := range mfes {
		if mfe.Subresource != subresource {
			continue
		}
		if mfe.FieldsType != "FieldsV1" || mfe.FieldsV1 == nil {
			panic(mfe)
		}
		reader := bytes.NewReader(mfe.FieldsV1.Raw)
		fields := fieldpath.NewSet()
		err := fields.FromJSON(reader)
		if err != nil {
			panic(err)
		}
		mfeCopy := mfe
		mfeCopy.FieldsV1 = nil
		ans = append(ans, ManagedFieldMap{mfeCopy, fields})
	}
	return ans
}

type ManagedFieldMapStr struct {
	MFE    metav1.ManagedFieldsEntry // but with FieldsV1 empty
	Fields string                    // JSON format
}

func (fm FieldMap) MarshalLog() any {
	ans := []any{}
	for _, mfm := range fm {
		fieldsJSON, err := mfm.Fields.ToJSON()
		if err != nil {
			ans = append(ans, err.Error())
		} else {
			ans = append(ans, ManagedFieldMapStr{mfm.MFE, string(fieldsJSON)})
		}
	}
	return ans
}

func (fm FieldMap) Step(pathElement fieldpath.PathElement) FieldMap {
	ans := []ManagedFieldMap{}
	for _, mfm := range fm {
		fieldsNext := mfm.Fields.WithPrefix(pathElement)
		if !fieldsNext.Empty() {
			ans = append(ans, ManagedFieldMap{mfm.MFE, fieldsNext})
		}
	}
	return ans
}

func (fm FieldMap) Advance(path fieldpath.Path) FieldMap {
	ans := fm
	for _, elt := range path {
		ans = ans.Step(elt)
	}
	return ans
}

func (fm FieldMap) Empty() bool {
	for _, mfm := range fm {
		if !mfm.Fields.Empty() {
			return false
		}
	}
	return true
}
