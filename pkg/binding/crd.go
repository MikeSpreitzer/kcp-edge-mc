/*
Copyright 2023 The KubeStellar Authors.

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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/kubestellar/kubestellar/pkg/util"
)

type APIResource struct {
	groupVersion schema.GroupVersion
	resource     metav1.APIResource
}

// Handle CRDs should account for CRDs being added or deleted to start/stop new informers as needed
func (c *Controller) handleCRD(ctx context.Context, _ util.ObjectIdentifier) error {
	logger := klog.FromContext(ctx)

	toStartList, toStopList, err := c.checkAPIResourcesForUpdates(ctx)
	if err != nil {
		return err
	}

	go c.startInformersForNewAPIResources(ctx, toStartList)

	for _, gvk := range toStopList {
		logger.Info("API removed, stopping informer.", "gvk", gvk)
		stopper := c.stoppers[gvk]
		// close channel
		close(stopper)
		// remove entries for key
		delete(c.informers, gvk)
		delete(c.listers, gvk)
		delete(c.stoppers, gvk)
		c.gvkToGvrMapper.Delete(gvk)
	}

	return nil
}

// checks what APIs need starting new informers or stopping informers.
// Returns a list of APIResources for informers to start and a list of keys for infomers to stop
func (c *Controller) checkAPIResourcesForUpdates(ctx context.Context) ([]APIResource, []schema.GroupVersionKind,
	error) {
	logger := klog.FromContext(ctx)

	toStart := []APIResource{}
	toStop := []schema.GroupVersionKind{}

	// tracking keys are used to detect what API resources have been removed
	trackingKeys := sets.Set[schema.GroupVersionKind]{}
	for k := range c.informers {
		trackingKeys.Insert(k)
	}

	// Get all the api resources in the cluster
	apiResources, err := c.kubernetesClient.Discovery().ServerPreferredResources()
	if err != nil {
		// ignore the error caused by a stale API service
		if !strings.Contains(err.Error(), util.UnableToRetrieveCompleteAPIListError) {
			return nil, nil, err
		}
	}

	// Loop through the api resources and create informers and listers for each of them
	for _, list := range apiResources {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			logger.Error(err, "Failed to parse a GroupVersion", "groupVersion", list.GroupVersion)
			continue
		}
		if _, excluded := excludedGroups[gv.Group]; excluded {
			continue
		}
		for _, resource := range list.APIResources {
			if _, excluded := excludedResourceNames[resource.Name]; excluded {
				continue
			}
			if !util.IsAPIGroupAllowed(gv.Group, c.allowedGroupsSet) {
				continue
			}
			informable := verbsSupportInformers(resource.Verbs)
			if informable {
				key := gv.WithKind(resource.Kind)
				if _, ok := c.informers[key]; !ok {
					toStart = append(toStart, APIResource{
						groupVersion: gv,
						resource:     resource,
					})
				}
				// remove the key from tracking keys, what is left in the map at the end are
				// keys to the informers that need to be stopped.
				delete(trackingKeys, key)
			}
		}
	}

	for k := range trackingKeys {
		toStop = append(toStop, k)
	}

	return toStart, toStop, nil
}

func (c *Controller) startInformersForNewAPIResources(ctx context.Context, toStartList []APIResource) {
	logger := klog.FromContext(ctx)

	for _, toStart := range toStartList {
		logger.Info("New API added. Starting informer for:", "group", toStart.groupVersion.Group,
			"version", toStart.groupVersion, "kind", toStart.resource.Kind)

		gvr := schema.GroupVersionResource{
			Group:    toStart.groupVersion.Group,
			Version:  toStart.groupVersion.Version,
			Resource: toStart.resource.Name,
		}

		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return c.dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return c.dynamicClient.Resource(gvr).Watch(context.TODO(), metav1.ListOptions{})
				},
			},
			nil,
			0, //Skip resync
			cache.Indexers{},
		)

		// add the event handler functions (same as those used by the startup logic)
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: c.handleObject,
			UpdateFunc: func(old, new interface{}) {
				if shouldSkipUpdate(old, new) {
					return
				}
				c.handleObject(new)
			},
			DeleteFunc: func(obj interface{}) {
				c.handleObject(obj)
			},
		})
		key := toStart.groupVersion.WithKind(toStart.resource.Kind)
		c.informers[key] = informer

		// add the mapping between GVK and GVR
		c.gvkToGvrMapper.Add(toStart.groupVersion.WithKind(toStart.resource.Kind), gvr)

		// create and index the lister
		lister := cache.NewGenericLister(informer.GetIndexer(), gvr.GroupResource())
		c.listers[key] = lister
		stopper := make(chan struct{})
		defer close(stopper)
		c.stoppers[key] = stopper

		go informer.Run(stopper)
	}
	// block
	select {}
}
