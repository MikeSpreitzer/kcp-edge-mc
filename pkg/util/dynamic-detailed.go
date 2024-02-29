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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

var versionV1 = schema.GroupVersion{Version: "v1"}
var parameterScheme = MakeMetaV1Scheme()
var dynamicParameterCodec = runtime.NewParameterCodec(parameterScheme)

func MakeMetaV1Scheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	metav1.AddMetaToScheme(scheme)
	metav1.AddToGroupVersion(scheme, versionV1)
	return scheme
}

func GetWithManagedFields(ctx context.Context, restClient rest.Interface, gvr schema.GroupVersionResource, namespace, name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	url := []string{}
	if len(gvr.Group) == 0 {
		url = append(url, "api")
	} else {
		url = append(url, "apis", gvr.Group)
	}
	url = append(url, gvr.Version)
	if len(namespace) > 0 {
		url = append(url, "namespaces", namespace)
	}
	url = append(url, gvr.Resource)
	if len(name) > 0 {
		url = append(url, name)
	}
	url = append(url, subresources...)
	req := restClient.Get()
	req = req.AbsPath(url...)
	req = req.SpecificallyVersionedParams(&opts, dynamicParameterCodec, versionV1)
	result := req.Do(ctx)
	if err := result.Error(); err != nil {
		return nil, err
	}
	retBytes, err := result.Raw()
	if err != nil {
		return nil, err
	}
	uncastObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, retBytes)
	if err != nil {
		return nil, err
	}
	return uncastObj.(*unstructured.Unstructured), nil
}
