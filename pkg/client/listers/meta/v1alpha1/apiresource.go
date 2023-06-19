//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright The KubeStellar Authors.

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

// Code generated by kcp code-generator. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"

	metav1alpha1 "github.com/kcp-dev/edge-mc/pkg/apis/meta/v1alpha1"
)

// APIResourceClusterLister can list APIResources across all workspaces, or scope down to a APIResourceLister for one workspace.
// All objects returned here must be treated as read-only.
type APIResourceClusterLister interface {
	// List lists all APIResources in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*metav1alpha1.APIResource, err error)
	// Cluster returns a lister that can list and get APIResources in one workspace.
	Cluster(clusterName logicalcluster.Name) APIResourceLister
	APIResourceClusterListerExpansion
}

type aPIResourceClusterLister struct {
	indexer cache.Indexer
}

// NewAPIResourceClusterLister returns a new APIResourceClusterLister.
// We assume that the indexer:
// - is fed by a cross-workspace LIST+WATCH
// - uses kcpcache.MetaClusterNamespaceKeyFunc as the key function
// - has the kcpcache.ClusterIndex as an index
func NewAPIResourceClusterLister(indexer cache.Indexer) *aPIResourceClusterLister {
	return &aPIResourceClusterLister{indexer: indexer}
}

// List lists all APIResources in the indexer across all workspaces.
func (s *aPIResourceClusterLister) List(selector labels.Selector) (ret []*metav1alpha1.APIResource, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*metav1alpha1.APIResource))
	})
	return ret, err
}

// Cluster scopes the lister to one workspace, allowing users to list and get APIResources.
func (s *aPIResourceClusterLister) Cluster(clusterName logicalcluster.Name) APIResourceLister {
	return &aPIResourceLister{indexer: s.indexer, clusterName: clusterName}
}

// APIResourceLister can list all APIResources, or get one in particular.
// All objects returned here must be treated as read-only.
type APIResourceLister interface {
	// List lists all APIResources in the workspace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*metav1alpha1.APIResource, err error)
	// Get retrieves the APIResource from the indexer for a given workspace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*metav1alpha1.APIResource, error)
	APIResourceListerExpansion
}

// aPIResourceLister can list all APIResources inside a workspace.
type aPIResourceLister struct {
	indexer     cache.Indexer
	clusterName logicalcluster.Name
}

// List lists all APIResources in the indexer for a workspace.
func (s *aPIResourceLister) List(selector labels.Selector) (ret []*metav1alpha1.APIResource, err error) {
	err = kcpcache.ListAllByCluster(s.indexer, s.clusterName, selector, func(i interface{}) {
		ret = append(ret, i.(*metav1alpha1.APIResource))
	})
	return ret, err
}

// Get retrieves the APIResource from the indexer for a given workspace and name.
func (s *aPIResourceLister) Get(name string) (*metav1alpha1.APIResource, error) {
	key := kcpcache.ToClusterAwareKey(s.clusterName.String(), "", name)
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(metav1alpha1.Resource("APIResource"), name)
	}
	return obj.(*metav1alpha1.APIResource), nil
}

// NewAPIResourceLister returns a new APIResourceLister.
// We assume that the indexer:
// - is fed by a workspace-scoped LIST+WATCH
// - uses cache.MetaNamespaceKeyFunc as the key function
func NewAPIResourceLister(indexer cache.Indexer) *aPIResourceScopedLister {
	return &aPIResourceScopedLister{indexer: indexer}
}

// aPIResourceScopedLister can list all APIResources inside a workspace.
type aPIResourceScopedLister struct {
	indexer cache.Indexer
}

// List lists all APIResources in the indexer for a workspace.
func (s *aPIResourceScopedLister) List(selector labels.Selector) (ret []*metav1alpha1.APIResource, err error) {
	err = cache.ListAll(s.indexer, selector, func(i interface{}) {
		ret = append(ret, i.(*metav1alpha1.APIResource))
	})
	return ret, err
}

// Get retrieves the APIResource from the indexer for a given workspace and name.
func (s *aPIResourceScopedLister) Get(name string) (*metav1alpha1.APIResource, error) {
	key := name
	obj, exists, err := s.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(metav1alpha1.Resource("APIResource"), name)
	}
	return obj.(*metav1alpha1.APIResource), nil
}