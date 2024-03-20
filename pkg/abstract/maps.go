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

package abstract

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/util/sets"
)

// Map is a finite set of (key,value) pairs
// that has at most one value for any given key.
// The keys are comparable in some way, not necessarily by Go's `==`.
// The collection may or may not be mutable.
// This view of the collection may or may not have a limited scope of validity.
// This view may or may not have concurrency restrictions.
type Map[Key, Val any] interface {
	Iterable2[Key, Val]
	Getter[Key, Val]
	Len() int
}

// LangMap is a Map implemented by a golang `map`.
type LangMap[Key comparable, Val any] map[Key]Val

var _ Map[string, func()] = LangMap[string, func()]{}
var _ logr.Marshaler = LangMap[string, func()]{}

// NewLangMap makes a new empty LangMap
func NewLangMap[Key comparable, Val any]() LangMap[Key, Val] {
	return LangMap[Key, Val]{}
}

// MapFromLang recognizes an existing `map` as a LangMap
func MapFromLang[Key comparable, Val any](base map[Key]Val) LangMap[Key, Val] {
	return LangMap[Key, Val](base)
}

// LangMapCopyOf makes a new LangMap that holds a copy of the given Map
func LangMapCopyOf[Key comparable, Val any](base Map[Key, Val]) LangMap[Key, Val] {
	ans := NewLangMap[Key, Val]()
	base.Iterator2()(func(key Key, val Val) bool {
		ans[key] = val
		return true
	})
	return ans
}

func (lm LangMap[Key, Val]) Len() int { return len(lm) }

func (lm LangMap[Key, Val]) Get(key Key) (Val, bool) {
	ans, have := lm[key]
	return ans, have
}

func (lm LangMap[Key, Val]) Iterator2() Iterator2[Key, Val] {
	return func(yield func(key Key, val Val) bool) {
		for key, val := range lm {
			if !yield(key, val) {
				return
			}
		}
	}
}

// MarshalLog makes LangMap implement logr.Marshaler
func (lm LangMap[Key, Val]) MarshalLog() any {
	return MapMarshalLog[Key, Val](lm)
}

// MapMarshalLog converts the given Map to something that logr likes
func MapMarshalLog[Key, Val any](input Map[Key, Val]) any {
	forLog := make(map[string]any, input.Len())
	input.Iterator2()(func(key Key, val Val) bool {
		keyBytes, err := json.Marshal(key)
		if err != nil {
			keyBytes = []byte(fmt.Sprintf("%#v", key))
		}
		forLog[string(keyBytes)] = val
		return true
	})
	return forLog
}

// MapK8sSetToConstant maps every member of a Set to the same value
type MapK8sSetToConstant[Key comparable, Val any] struct {
	Set sets.Set[Key]
	Val Val
}

var _ Map[string, func()] = MapK8sSetToConstant[string, func()]{}
var _ logr.Marshaler = MapK8sSetToConstant[string, func()]{}

// NewMapK8sSetToConstant makes a Map that maps every member of the given set to the one same given value
func NewMapK8sSetToConstant[Key comparable, Val any](set sets.Set[Key], val Val) MapK8sSetToConstant[Key, Val] {
	return MapK8sSetToConstant[Key, Val]{set, val}
}

func (stc MapK8sSetToConstant[Key, Val]) Len() int { return stc.Set.Len() }

func (stc MapK8sSetToConstant[Key, Val]) Get(key Key) (Val, bool) {
	if stc.Set.Has(key) {
		return stc.Val, true
	}
	var zero Val
	return zero, false
}

func (stc MapK8sSetToConstant[Key, Val]) Iterator2() Iterator2[Key, Val] {
	return func(yield func(key Key, val Val) bool) {
		for key := range stc.Set {
			if !yield(key, stc.Val) {
				return
			}
		}
	}
}

// MarshalLog makes MapK8sSetToConstant implement logr.Marshaler
func (stc MapK8sSetToConstant[Key, Val]) MarshalLog() any {
	return MapMarshalLog[Key, Val](stc)
}
