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

package binding

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	k8score "k8s.io/api/core/v1"
	k8snetv1 "k8s.io/api/networking/v1"
	k8snetv1b1 "k8s.io/api/networking/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/kubernetes/test/integration/framework"

	ksapi "github.com/kubestellar/kubestellar/api/control/v1alpha1"
	ksclient "github.com/kubestellar/kubestellar/pkg/generated/clientset/versioned/typed/control/v1alpha1"

	"github.com/kubestellar/kubestellar/pkg/util"
)

func TestController(t *testing.T) {
	rg := rand.New(rand.NewSource(42))
	rg.Uint64()
	rg.Uint64()
	rg.Uint64()
	logger, ctx := ktesting.NewTestContext(t)
	nObj := 3
	for trial := 1; trial <= 2; trial++ {
		ctx, cancel := context.WithCancel(ctx)
		client, config, teardown := framework.StartTestServer(ctx, t, framework.TestServerSetup{})
		fullTeardwon := func() {
			cancel()
			teardown()
		}
		ksClient, err := ksclient.NewForConfig(config)
		if err != nil {
			t.Fatalf("Failed to create KubeStellar client: %w", err)
		}
		namespaces := []*k8score.Namespace{
			generateNamespace(t, ctx, rg, "ns1", client),
			generateNamespace(t, ctx, rg, "ns2", client),
			generateNamespace(t, ctx, rg, "ns3", client),
		}
		objs := make([]mrObjRsc, nObj)
		for i := 0; i < nObj; i++ {
			thisClient := client
			if i*3 >= nObj*2 { // do not actually create the last third
				thisClient = nil
			}
			objs[i] = generateObject(t, ctx, rg, 0, namespaces, thisClient)
		}
		tests := []ksapi.DownsyncObjectTest{}
		for i := nObj / 3; i < nObj; i++ { // request downsync of the last 2/3
			tests = append(tests, extractTest(rg, objs[i]))
		}
		expectedObjRefs := sets.New[util.Key]()
		for i := 0; i*3 < nObj*2; i++ { // any of the first 2/3 might match a test
			if objs[i].MatchesAny(t, tests) {
				key, err := util.KeyForGroupVersionKindNamespaceName(objs[i].mrObject)
				if err != nil {
					t.Fatalf("Failed to extract Key from %#v: %w", objs[i].mrObject, err)
				}
				expectedObjRefs.Insert(key)
			}
		}
		logger.Info("Generated mrObjRscs", "objs", objs)
		bp := &ksapi.BindingPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("trial%d", trial),
			},
			Spec: ksapi.BindingPolicySpec{
				Downsync: tests,
			},
		}
		_, err = ksClient.BindingPolicies().Create(ctx, bp, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create BidingPolicy: %w", err)
		}
		ctlr, err := NewController(logger, config, config, "test-wds", nil)
		if err != nil {
			t.Fatalf("Failed to create controller: %w", err)
		}
		err = ctlr.EnsureCRDs(ctx)
		if err != nil {
			t.Fatal(err)
		}
		ctlr.Start(ctx, 4)
		fullTeardwon()
	}
}

type mrObjRsc struct {
	mrObject
	resource  string
	namespace *k8score.Namespace
}

func (mor mrObjRsc) MatchesAny(t *testing.T, tests []ksapi.DownsyncObjectTest) bool {
	for _, test := range tests {
		gvk := mor.GetObjectKind().GroupVersionKind()
		if test.APIGroup != nil && gvk.Group != *test.APIGroup {
			continue
		}
		if len(test.Resources) > 0 && !(SliceContains(test.Resources, mor.resource) || SliceContains(test.Resources, "*")) {
			continue
		}
		if len(test.Namespaces) > 0 && !(SliceContains(test.Namespaces, mor.GetNamespace()) || SliceContains(test.Namespaces, "*")) {
			continue
		}
		if len(test.ObjectNames) > 0 && !(SliceContains(test.ObjectNames, mor.GetName()) || SliceContains(test.ObjectNames, "*")) {
			continue
		}
		if len(test.NamespaceSelectors) > 0 && !LabelsMatchAny(t, mor.namespace.Labels, test.NamespaceSelectors) {
			continue
		}
		if len(test.ObjectSelectors) > 0 && !LabelsMatchAny(t, mor.GetLabels(), test.ObjectSelectors) {
			continue
		}
	}
	return false
}

func LabelsMatchAny(t *testing.T, labels map[string]string, selectors []metav1.LabelSelector) bool {
	for _, ls := range selectors {
		sel, err := metav1.LabelSelectorAsSelector(&ls)
		if err != nil {
			t.Fatalf("Failed to convert LabelSelector %#v to labels.Selector: %w", ls, err)
			continue
		}
		if sel.Matches(k8slabels.Set(labels)) {
			return true
		}
	}
	return false

}

func extractTest(rg *rand.Rand, object mrObjRsc) ksapi.DownsyncObjectTest {
	ans := ksapi.DownsyncObjectTest{}
	if rg.Intn(10) < 7 {
		group := object.GetObjectKind().GroupVersionKind().Group
		ans.APIGroup = &group
	}
	ans.Resources = extractStringTest(rg, object.resource)
	if object.namespace != nil {
		ans.Namespaces = extractStringTest(rg, object.GetNamespace())
		ans.NamespaceSelectors = extractLabelsTest(rg, object.namespace.Labels)
	}
	ans.ObjectNames = extractStringTest(rg, object.GetName())
	ans.ObjectSelectors = extractLabelsTest(rg, object.GetLabels())
	return ans
}

func extractStringTest(rg *rand.Rand, good string) []string {
	ans := []string{}
	if rg.Intn(10) < 2 {
		ans = append(ans, "foo")
	}
	if rg.Intn(10) < 7 {
		ans = append(ans, good)
	}
	if rg.Intn(10) < 2 {
		ans = append(ans, "bar")
	}
	return ans
}

func extractLabelsTest(rg *rand.Rand, goodLabels map[string]string) []metav1.LabelSelector {
	testLabels := map[string]string{}
	if rg.Intn(10) < 2 {
		testLabels["foo"] = "bar"
	}
	for key, val := range goodLabels {
		if rg.Intn(10) < 5 {
			continue
		}
		testVal := val
		if rg.Intn(10) < 2 {
			testVal = val + "not"
		}
		testLabels[key] = testVal
	}
	return []metav1.LabelSelector{{MatchLabels: testLabels}}
}

func getObjectTest(rg *rand.Rand, apiGroups []string, resources []string, namespaces []*k8score.Namespace, objects []mrObject) ksapi.DownsyncObjectTest {
	ans := ksapi.DownsyncObjectTest{}
	if rg.Intn(10) < 7 {
		ans.APIGroup = &apiGroups[rg.Intn(len(apiGroups))]
	}
	ans.Resources = make([]string, rg.Intn(3))
	baseJ := 0
	for i := range ans.Resources {
		// Leave room for len(ans.Resources) - (i+1) more
		// pick an index in [baseJ, len(resources) + i+1 - len(ans.Resources))
		j := baseJ + rg.Intn(len(resources)+i+1-len(ans.Resources)-baseJ)
		ans.Resources[i] = resources[j]
		baseJ = j + 1
	}
	ans.Namespaces = make([]string, rg.Intn(3))
	baseJ = 0
	for i := range ans.Namespaces {
		j := baseJ + rg.Intn(len(namespaces)+i+1-len(ans.Namespaces)-baseJ)
		ans.Namespaces[i] = namespaces[j].Name
		baseJ = j + 1
	}
	if rg.Intn(2) == 0 {
		i := rg.Intn(len(namespaces))
		ans.NamespaceSelectors = []metav1.LabelSelector{{MatchLabels: namespaces[i].Labels}}
	}
	ans.ObjectNames = make([]string, rg.Intn(3))
	baseJ = 0
	for i := range ans.ObjectNames {
		j := baseJ + rg.Intn(len(objects)+i+1-len(ans.ObjectNames)-baseJ)
		ans.ObjectNames[i] = objects[j].GetName()
		baseJ = j + 1
	}
	return ans
}

func generateLabels(rg *rand.Rand) map[string]string {
	ans := map[string]string{}
	n := 1 + rg.Intn(2)
	for i := 1; i <= n; i++ {
		ans[fmt.Sprintf("l%d", i*10+rg.Intn(2))] = fmt.Sprintf("v%d", i*10+rg.Intn(2))
	}
	return ans
}

func generateObjectMeta(rg *rand.Rand, name string, namespace *k8score.Namespace) metav1.ObjectMeta {
	ans := metav1.ObjectMeta{
		Name:   name,
		Labels: generateLabels(rg),
	}
	if namespace != nil {
		ans.Namespace = namespace.Name
	}
	return ans
}

func generateNamespace(t *testing.T, ctx context.Context, rg *rand.Rand, name string, client k8sclient.Interface) *k8score.Namespace {
	ans := &k8score.Namespace{
		TypeMeta:   metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"},
		ObjectMeta: generateObjectMeta(rg, name, nil),
	}
	_, err := client.CoreV1().Namespaces().Create(ctx, ans, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Namespace %#v: %w", ans, err)
	}
	return ans
}

func generateObject(t *testing.T, ctx context.Context, rg *rand.Rand, index int, namespaces []*k8score.Namespace, client k8sclient.Interface) mrObjRsc {
	x := rg.Intn(40)
	namespace := namespaces[rg.Intn(len(namespaces))]
	var err error
	var ans mrObjRsc
	switch {
	case x < 10:
		obj := &k8score.ConfigMap{
			TypeMeta:   typeMeta("ConfigMap", k8score.SchemeGroupVersion),
			ObjectMeta: generateObjectMeta(rg, fmt.Sprintf("o%d", index), namespace),
		}
		if client != nil {
			_, err = client.CoreV1().ConfigMaps(obj.Namespace).Create(ctx, obj, metav1.CreateOptions{})
		}
		ans = mrObjRsc{obj, "configmaps", namespace}
	case x < 20:
		obj := &rbacv1.ClusterRole{
			TypeMeta:   typeMeta("ClusterRole", rbacv1.SchemeGroupVersion),
			ObjectMeta: generateObjectMeta(rg, fmt.Sprintf("o%d", index), nil),
		}
		if client != nil {
			_, err = client.RbacV1().ClusterRoles().Create(ctx, obj, metav1.CreateOptions{})
		}
		ans = mrObjRsc{obj, "clusterroles", nil}
	case x < 30:
		obj := &k8snetv1.NetworkPolicy{
			TypeMeta:   typeMeta("NetworkPolicy", k8snetv1.SchemeGroupVersion),
			ObjectMeta: generateObjectMeta(rg, fmt.Sprintf("o%d", index), namespace),
		}
		if client != nil {
			_, err = client.NetworkingV1().NetworkPolicies(obj.Namespace).Create(ctx, obj, metav1.CreateOptions{})
		}
		ans = mrObjRsc{obj, "networkpolicies", namespace}
	default:
		obj := &k8snetv1b1.IngressClass{
			TypeMeta:   typeMeta("IngressClass", k8snetv1b1.SchemeGroupVersion),
			ObjectMeta: generateObjectMeta(rg, fmt.Sprintf("o%d", index), nil),
		}
		if client != nil {
			_, err = client.NetworkingV1beta1().IngressClasses().Create(ctx, obj, metav1.CreateOptions{})
		}
		ans = mrObjRsc{obj, "ingressclasses", nil}
	}
	if err != nil {
		t.Fatalf("Failed to create object %#v: %w", ans.mrObject, err)
	}
	return ans
}

func typeMeta(kind string, groupVersion k8sschema.GroupVersion) metav1.TypeMeta {
	return metav1.TypeMeta{Kind: kind, APIVersion: groupVersion.String()}
}
