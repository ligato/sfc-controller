// Copyright (c) 2018 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/ligato/sfc-controller/plugins/k8scrd/pkg/apis/sfccontroller/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeNetworkNodeOverlays implements NetworkNodeOverlayInterface
type FakeNetworkNodeOverlays struct {
	Fake *FakeSfccontrollerV1alpha1
	ns   string
}

var networknodeoverlaysResource = schema.GroupVersionResource{Group: "sfccontroller.ligato.github.com", Version: "v1alpha1", Resource: "networknodeoverlays"}

var networknodeoverlaysKind = schema.GroupVersionKind{Group: "sfccontroller.ligato.github.com", Version: "v1alpha1", Kind: "NetworkNodeOverlay"}

// Get takes name of the networkNodeOverlay, and returns the corresponding networkNodeOverlay object, and an error if there is any.
func (c *FakeNetworkNodeOverlays) Get(name string, options v1.GetOptions) (result *v1alpha1.NetworkNodeOverlay, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(networknodeoverlaysResource, c.ns, name), &v1alpha1.NetworkNodeOverlay{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NetworkNodeOverlay), err
}

// List takes label and field selectors, and returns the list of NetworkNodeOverlays that match those selectors.
func (c *FakeNetworkNodeOverlays) List(opts v1.ListOptions) (result *v1alpha1.NetworkNodeOverlayList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(networknodeoverlaysResource, networknodeoverlaysKind, c.ns, opts), &v1alpha1.NetworkNodeOverlayList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.NetworkNodeOverlayList{}
	for _, item := range obj.(*v1alpha1.NetworkNodeOverlayList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested networkNodeOverlays.
func (c *FakeNetworkNodeOverlays) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(networknodeoverlaysResource, c.ns, opts))

}

// Create takes the representation of a networkNodeOverlay and creates it.  Returns the server's representation of the networkNodeOverlay, and an error, if there is any.
func (c *FakeNetworkNodeOverlays) Create(networkNodeOverlay *v1alpha1.NetworkNodeOverlay) (result *v1alpha1.NetworkNodeOverlay, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(networknodeoverlaysResource, c.ns, networkNodeOverlay), &v1alpha1.NetworkNodeOverlay{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NetworkNodeOverlay), err
}

// Update takes the representation of a networkNodeOverlay and updates it. Returns the server's representation of the networkNodeOverlay, and an error, if there is any.
func (c *FakeNetworkNodeOverlays) Update(networkNodeOverlay *v1alpha1.NetworkNodeOverlay) (result *v1alpha1.NetworkNodeOverlay, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(networknodeoverlaysResource, c.ns, networkNodeOverlay), &v1alpha1.NetworkNodeOverlay{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NetworkNodeOverlay), err
}

// Delete takes name of the networkNodeOverlay and deletes it. Returns an error if one occurs.
func (c *FakeNetworkNodeOverlays) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(networknodeoverlaysResource, c.ns, name), &v1alpha1.NetworkNodeOverlay{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeNetworkNodeOverlays) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(networknodeoverlaysResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.NetworkNodeOverlayList{})
	return err
}

// Patch applies the patch and returns the patched networkNodeOverlay.
func (c *FakeNetworkNodeOverlays) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.NetworkNodeOverlay, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(networknodeoverlaysResource, c.ns, name, data, subresources...), &v1alpha1.NetworkNodeOverlay{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NetworkNodeOverlay), err
}
