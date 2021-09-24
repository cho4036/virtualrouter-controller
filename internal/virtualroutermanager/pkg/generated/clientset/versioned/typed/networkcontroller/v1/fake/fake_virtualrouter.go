/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	networkcontrollerv1 "github.com/cho4036/virtualrouter-controller/internal/virtualroutermanager/pkg/apis/networkcontroller/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeVirtualRouters implements VirtualRouterInterface
type FakeVirtualRouters struct {
	Fake *FakeTmaxV1
	ns   string
}

var virtualroutersResource = schema.GroupVersionResource{Group: "tmax.hypercloud.com", Version: "v1", Resource: "virtualrouters"}

var virtualroutersKind = schema.GroupVersionKind{Group: "tmax.hypercloud.com", Version: "v1", Kind: "VirtualRouter"}

// Get takes name of the virtualRouter, and returns the corresponding virtualRouter object, and an error if there is any.
func (c *FakeVirtualRouters) Get(ctx context.Context, name string, options v1.GetOptions) (result *networkcontrollerv1.VirtualRouter, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(virtualroutersResource, c.ns, name), &networkcontrollerv1.VirtualRouter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*networkcontrollerv1.VirtualRouter), err
}

// List takes label and field selectors, and returns the list of VirtualRouters that match those selectors.
func (c *FakeVirtualRouters) List(ctx context.Context, opts v1.ListOptions) (result *networkcontrollerv1.VirtualRouterList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(virtualroutersResource, virtualroutersKind, c.ns, opts), &networkcontrollerv1.VirtualRouterList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &networkcontrollerv1.VirtualRouterList{ListMeta: obj.(*networkcontrollerv1.VirtualRouterList).ListMeta}
	for _, item := range obj.(*networkcontrollerv1.VirtualRouterList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested virtualRouters.
func (c *FakeVirtualRouters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(virtualroutersResource, c.ns, opts))

}

// Create takes the representation of a virtualRouter and creates it.  Returns the server's representation of the virtualRouter, and an error, if there is any.
func (c *FakeVirtualRouters) Create(ctx context.Context, virtualRouter *networkcontrollerv1.VirtualRouter, opts v1.CreateOptions) (result *networkcontrollerv1.VirtualRouter, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(virtualroutersResource, c.ns, virtualRouter), &networkcontrollerv1.VirtualRouter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*networkcontrollerv1.VirtualRouter), err
}

// Update takes the representation of a virtualRouter and updates it. Returns the server's representation of the virtualRouter, and an error, if there is any.
func (c *FakeVirtualRouters) Update(ctx context.Context, virtualRouter *networkcontrollerv1.VirtualRouter, opts v1.UpdateOptions) (result *networkcontrollerv1.VirtualRouter, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(virtualroutersResource, c.ns, virtualRouter), &networkcontrollerv1.VirtualRouter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*networkcontrollerv1.VirtualRouter), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeVirtualRouters) UpdateStatus(ctx context.Context, virtualRouter *networkcontrollerv1.VirtualRouter, opts v1.UpdateOptions) (*networkcontrollerv1.VirtualRouter, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(virtualroutersResource, "status", c.ns, virtualRouter), &networkcontrollerv1.VirtualRouter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*networkcontrollerv1.VirtualRouter), err
}

// Delete takes name of the virtualRouter and deletes it. Returns an error if one occurs.
func (c *FakeVirtualRouters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(virtualroutersResource, c.ns, name), &networkcontrollerv1.VirtualRouter{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeVirtualRouters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(virtualroutersResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &networkcontrollerv1.VirtualRouterList{})
	return err
}

// Patch applies the patch and returns the patched virtualRouter.
func (c *FakeVirtualRouters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *networkcontrollerv1.VirtualRouter, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(virtualroutersResource, c.ns, name, pt, data, subresources...), &networkcontrollerv1.VirtualRouter{})

	if obj == nil {
		return nil, err
	}
	return obj.(*networkcontrollerv1.VirtualRouter), err
}
