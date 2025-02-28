// Copyright 2020 Google LLC.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// client is the client to update object in the API server.
type client struct {
	client     dynamic.Interface
	restMapper meta.RESTMapper
}

func NewClient(d dynamic.Interface, mapper meta.RESTMapper) *client {
	return &client{
		client:     d,
		restMapper: mapper,
	}
}

// Update updates an object using dynamic client
func (uc *client) Update(ctx context.Context, meta object.ObjMetadata, obj *unstructured.Unstructured, options *metav1.UpdateOptions) error {
	r, err := uc.resourceInterface(meta)
	if err != nil {
		return err
	}
	if options == nil {
		options = &metav1.UpdateOptions{}
	}
	_, err = r.Update(ctx, obj, *options)
	return err
}

// Get fetches the requested object into the input obj using dynamic client
func (uc *client) Get(ctx context.Context, meta object.ObjMetadata) (*unstructured.Unstructured, error) {
	r, err := uc.resourceInterface(meta)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, meta.Name, metav1.GetOptions{})
}

func (uc *client) resourceInterface(meta object.ObjMetadata) (dynamic.ResourceInterface, error) {
	mapping, err := uc.restMapper.RESTMapping(meta.GroupKind)
	if err != nil {
		return nil, err
	}
	namespacedClient := uc.client.Resource(mapping.Resource).Namespace(meta.Namespace)
	return namespacedClient, nil
}

// UpdateAnnotation updates the object owning inventory annotation
// to the new ID when the owning inventory annotation is either empty or the old ID.
// It returns if the annotation is updated.
func UpdateAnnotation(obj *unstructured.Unstructured, oldID, newID string) (bool, error) {
	key := "config.k8s.io/owning-inventory"
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	val, found := annotations[key]
	if !found || val == oldID {
		annotations[key] = newID
		// Since the annotation is updated, we also need to update the
		// last applied configuration annotation.
		u := getOriginalObj(obj)
		if u != nil {
			u.SetAnnotations(annotations)
			err := util.CreateOrUpdateAnnotation(false, u, scheme.DefaultJSONEncoder())
			obj.SetAnnotations(u.GetAnnotations())
			return true, err
		}
		obj.SetAnnotations(annotations)
		return true, nil
	}
	return false, nil
}

func getOriginalObj(obj *unstructured.Unstructured) *unstructured.Unstructured {
	annotations := obj.GetAnnotations()
	lastApplied, found := annotations[v1.LastAppliedConfigAnnotation]
	if !found {
		return nil
	}
	u := &unstructured.Unstructured{}
	err := json.Unmarshal([]byte(lastApplied), u)
	if err != nil {
		return nil
	}
	return u
}
