/*
Copyright 2017 The Kubernetes Authors.

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

package virtualroutermanager

import (
	"context"
	"fmt"
	"os"
	"time"

	samplev1alpha1 "github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/apis/networkcontroller/v1"
	clientset "github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/generated/clientset/versioned"
	samplescheme "github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/generated/clientset/versioned/scheme"
	informers "github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/generated/informers/externalversions/networkcontroller/v1"
	listers "github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/generated/listers/networkcontroller/v1"
	virtualrouter "github.com/tmax-cloud/virtualrouter/pkg/apis/networkcontroller"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"

	rbac_v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	v1Pod "k8s.io/kubernetes/pkg/api/v1/pod"
)

const controllerAgentName = "virtual-router"

var virtualRouterNamespace = "virtualrouter"

const (
	SERVICE_ACCOUNT_NAME = "virtualrouter-sa"
	ROLE_NAME            = "virtualrouter-role"
	ROLE_BINDING_NAME    = "virtualrouter-rb"
	VIRTUALROUTER_LABEL  = "virtualrouterInstance"

	VIRTUALROUTER_SCHEDULE_FINALIZER = "schedulerFinalizer"
)

type podKey string
type virtualrouterKey string

const (
	// SuccessSynced is used as part of the Event 'reason' when a VirtualRouter is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a VirtualRouter fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by VirtualRouter"
	// MessageResourceSynced is the message used for an Event fired when a VirtualRouter
	// is synced successfully
	MessageResourceSynced = "VirtualRouter synced successfully"
)

// Controller is the controller implementation for VirtualRouter resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	sampleclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced

	podLister corelisters.PodLister
	podSynced cache.InformerSynced

	virtualRoutersLister listers.VirtualRouterLister
	virtualRoutersSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	podInformer coreinformers.PodInformer,
	virtualRouterInformer informers.VirtualRouterInformer) *Controller {

	// Create event broadcaster
	// Add virtual-router types to the default Kubernetes Scheme so Events can be
	// logged for virtual-router types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:        kubeclientset,
		sampleclientset:      sampleclientset,
		deploymentsLister:    deploymentInformer.Lister(),
		deploymentsSynced:    deploymentInformer.Informer().HasSynced,
		podLister:            podInformer.Lister(),
		podSynced:            podInformer.Informer().HasSynced,
		virtualRoutersLister: virtualRouterInformer.Lister(),
		virtualRoutersSynced: virtualRouterInformer.Informer().HasSynced,
		workqueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "VirtualRouters"),
		recorder:             recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when VirtualRouter resources change
	virtualRouterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueVirtualRouter,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueVirtualRouter(new)
		},
		DeleteFunc: controller.enqueueVirtualRouter,
	})
	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a VirtualRouter resource will enqueue that VirtualRouter resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting VirtualRouter controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.virtualRoutersSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process VirtualRouter resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// VirtualRouter resource to be synced.
		if err := c.syncHandler(obj); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(obj)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the VirtualRouter resource
// with the current status of the resource.
func (c *Controller) syncHandler(obj interface{}) error {
	// Convert the namespace/name string into a distinct namespace and name
	switch key := obj.(type) {
	case podKey:
		var hasChanged bool = false

		namespace, name, err := cache.SplitMetaNamespaceKey(string(key))
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
			return nil
		}
		pod, err := c.podLister.Pods(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				controllerNS := os.Getenv("POD_NAMESPACE")
				virtualRouterList, err := c.virtualRoutersLister.VirtualRouters(controllerNS).List(nil)
				if err != nil {
					return err
				}
				var virtualRouter *samplev1alpha1.VirtualRouter
				for _, vr := range virtualRouterList {
					if vr.Name == pod.Namespace {
						virtualRouter = vr
					}
				}
				virtualRouterCopy := virtualRouter.DeepCopy()
				for _, st := range virtualRouterCopy.Status.ReplicaStatus {
					if st.PodName == pod.Name {
						if st.Phase == string(samplev1alpha1.REMOVED) {
							st.Phase = string(samplev1alpha1.REMOVED)
							hasChanged = true
						}
					}
				}
				if hasChanged {
					_, err := c.sampleclientset.TmaxV1().VirtualRouters(virtualRouter.Namespace).Update(context.TODO(), virtualRouterCopy, metav1.UpdateOptions{})
					if err != nil {
						return err
					}
				}
				return nil
			}
			return err
		}

		crName := pod.GetAnnotations()["customresourceName"]
		crNS := pod.GetAnnotations()["customresourceNamespace"]

		if crName == "" || crNS == "" {
			return fmt.Errorf("pod annotaion staled")
		}

		virtualRouter, err := c.virtualRoutersLister.VirtualRouters(crNS).Get(crName)
		if err != nil {
			if errors.IsNotFound(err) {
				utilruntime.HandleError(fmt.Errorf("virtualRouter '%s' in work queue no longer exists", key))
				return nil
			}
			return err
		}

		needToAdd := true
		virtualRouterCopy := virtualRouter.DeepCopy()
		var status *samplev1alpha1.ReplicaStatus

		for _, st := range virtualRouterCopy.Status.ReplicaStatus {
			if st.PodName == pod.Name {
				needToAdd = false
				status = &st
			}
		}

		if needToAdd {
			status = &samplev1alpha1.ReplicaStatus{
				PodName:   pod.Name,
				NodeName:  pod.Spec.NodeName,
				Bridged:   false,
				Scheduled: true,
				Phase:     string(samplev1alpha1.SCHEDULING),
			}
			virtualRouterCopy.Status.ReplicaStatus = append(virtualRouterCopy.Status.ReplicaStatus, *status)
			hasChanged = true
		} else {
			if v1Pod.IsPodReady(pod) {
				if !status.Scheduled || status.Phase != string(samplev1alpha1.SCHEDULED) {
					hasChanged = true
					status.Scheduled = true
					status.Phase = string(samplev1alpha1.SCHEDULED)
				}
			}
		}
		if hasChanged {
			_, err := c.sampleclientset.TmaxV1().VirtualRouters(virtualRouter.Namespace).Update(context.TODO(), virtualRouterCopy, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}

		return nil

	case virtualrouterKey:
		namespace, name, err := cache.SplitMetaNamespaceKey(string(key))
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
			return nil
		}

		// Get the VirtualRouter resource with this namespace/name
		virtualRouter, err := c.virtualRoutersLister.VirtualRouters(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				utilruntime.HandleError(fmt.Errorf("virtualRouter '%s' in work queue no longer exists", key))
				return nil
			}
			return err
		}

		// Deletion
		if !virtualRouter.ObjectMeta.DeletionTimestamp.IsZero() {
			if !containString(virtualRouter.Finalizers, VIRTUALROUTER_SCHEDULE_FINALIZER) {
				return nil
			}

			virtualRouterCopy := virtualRouter.DeepCopy()
			for i, status := range virtualRouterCopy.Status.ReplicaStatus {
				if status.Phase != string(samplev1alpha1.REMOVING) {
					virtualRouterCopy.Status.ReplicaStatus[i].Phase = string(samplev1alpha1.REMOVING)
				}
			}
			virtualRouter, err := c.sampleclientset.TmaxV1().VirtualRouters(virtualRouter.Namespace).Update(context.TODO(), virtualRouterCopy, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			deployment, err := c.deploymentsLister.Deployments(virtualRouter.Name).Get(virtualRouter.Spec.DeploymentName)
			if errors.IsNotFound(err) {
			} else {
				return err
			}
			err = c.kubeclientset.AppsV1().Deployments(deployment.Namespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}

			virtualRouterCopy = virtualRouter.DeepCopy()
			virtualRouterCopy.ObjectMeta.Finalizers = removeString(virtualRouter.Finalizers, VIRTUALROUTER_SCHEDULE_FINALIZER)
			_, err = c.sampleclientset.TmaxV1().VirtualRouters(virtualRouter.Namespace).Update(context.TODO(), virtualRouterCopy, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			return nil
		}

		deploymentName := virtualRouter.Spec.DeploymentName
		if deploymentName == "" {
			// We choose to absorb the error here as the worker would requeue the
			// resource otherwise. Instead, the next time the resource is updated
			// the resource will be queued again.
			utilruntime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
			return nil
		}

		// create deployment with new Namespace same as virtualrouter resource name
		newNS := virtualRouter.Name
		if err := c.ensureVirtualRouterNamespace(newNS, virtualRouter); err != nil {
			klog.Error(err)
			return err
		}

		if err := c.ensureVirtualRouterSA(newNS, virtualRouter); err != nil {
			klog.Error(err)
			return err
		}

		if err := c.ensureVirtualRouterRole(newNS, virtualRouter); err != nil {
			klog.Error(err)
			return err
		}

		if err := c.ensureVirtualRouterRoleBinding(newNS, virtualRouter); err != nil {
			klog.Error(err)
			return err
		}

		deployment, err := c.deploymentsLister.Deployments(newNS).Get(deploymentName)
		if errors.IsNotFound(err) {
			klog.Info("NotFound Deploy start")

			deployment, err = c.kubeclientset.AppsV1().Deployments(newNS).Create(context.TODO(), newDeployment(newNS, virtualRouter), metav1.CreateOptions{})
			if err != nil {
				return err
			}

			// // Initialize status
			// virtualRouterCopy := virtualRouter.DeepCopy()
			// initialStatus := samplev1alpha1.ReplicaStatus{
			// 	Scheduled: false,
			// 	PodName:   "",
			// 	HostName:  "",
			// 	Bridged:   false,
			// 	Phase:     string(samplev1alpha1.SCHEDULING),
			// }

			// for i := 0; i < int(*virtualRouterCopy.Spec.Replicas); i++ {
			// 	virtualRouterCopy.Status.ReplicaStatus = append(virtualRouterCopy.Status.ReplicaStatus, initialStatus)
			// }
			// _, err := c.sampleclientset.TmaxV1().VirtualRouters(virtualRouter.Namespace).Update(context.TODO(), virtualRouterCopy, metav1.UpdateOptions{})
			// if err != nil {
			// 	return err
			// }
		}

		if !metav1.IsControlledBy(deployment, virtualRouter) {
			msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
			c.recorder.Event(virtualRouter, corev1.EventTypeWarning, ErrResourceExists, msg)
			return fmt.Errorf(msg)
		}

		if virtualRouter.Spec.Replicas != nil && *virtualRouter.Spec.Replicas != *deployment.Spec.Replicas {
			klog.V(4).Infof("VirtualRouter %s replicas: %d, deployment replicas: %d", name, *virtualRouter.Spec.Replicas, *deployment.Spec.Replicas)
			deployment, err = c.kubeclientset.AppsV1().Deployments(newNS).Update(context.TODO(), newDeployment(newNS, virtualRouter), metav1.UpdateOptions{})

		}

		if err != nil {
			return err
		}

		err = c.updateVirtualRouterStatus(virtualRouter, deployment)
		if err != nil {
			return err
		}

		virtualRouter, err = c.virtualRoutersLister.VirtualRouters(namespace).Get(name)
		if err != nil {
			return err
		}
		for _, status := range virtualRouter.Status.ReplicaStatus {
			if status.Phase == string(samplev1alpha1.REMOVED) && !status.Bridged {

			}
		}

		c.recorder.Event(virtualRouter, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	}

	return nil
}

func (c *Controller) updateVirtualRouterStatus(virtualRouter *samplev1alpha1.VirtualRouter, deployment *appsv1.Deployment) error {
	virtualRouterCopy := virtualRouter.DeepCopy()
	virtualRouterCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	_, err := c.sampleclientset.TmaxV1().VirtualRouters(virtualRouter.Namespace).Update(context.TODO(), virtualRouterCopy, metav1.UpdateOptions{})
	return err
}

func (c *Controller) enqueueVirtualRouter(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the VirtualRouter resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that VirtualRouter resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a VirtualRouter, we should not do anything more
		// with it.
		if ownerRef.Kind != "VirtualRouter" {
			return
		}

		virtualRouter, err := c.virtualRoutersLister.VirtualRouters(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of virtualRouter '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueVirtualRouter(virtualRouter)
		return
	}
}

// newDeployment creates a new Deployment for a VirtualRouter resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the VirtualRouter resource that 'owns' it.
func newDeployment(newNS string, virtualRouter *samplev1alpha1.VirtualRouter) *appsv1.Deployment {
	labels := map[string]string{
		"app": VIRTUALROUTER_LABEL,
	}
	nodeSelectorMap := make(map[string]string)
	for _, nodeSelector := range virtualRouter.Spec.NodeSelector {
		nodeSelectorMap[nodeSelector.Key] = nodeSelector.Value
	}
	klog.Info(virtualRouter.Spec)
	klog.Info(virtualRouter.Spec.NodeSelector)
	klog.Info(nodeSelectorMap)
	// var uuid = uuid.Must(uuid.NewRandom())
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       virtualRouter.Spec.DeploymentName,
			Namespace:  newNS,
			Finalizers: []string{VIRTUALROUTER_SCHEDULE_FINALIZER},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(virtualRouter, samplev1alpha1.SchemeGroupVersion.WithKind("VirtualRouter")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: virtualRouter.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"customresourceName":      virtualRouter.Name,
						"customresourceNamespace": virtualRouter.Namespace,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "virtualrouter-sa",
					NodeSelector:       nodeSelectorMap,
					Containers: []corev1.Container{
						{
							// Name:            "virtualrouter-" + uuid.String(),
							Name:  virtualRouter.Name,
							Image: virtualRouter.Spec.Image,
							// Image:           "tmaxcloudck/virtualrouter:0.0.1",
							ImagePullPolicy: "Always",
							Env: []corev1.EnvVar{
								{
									Name:  "POD_NAMESPACE",
									Value: newNS,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										corev1.Capability("NET_RAW"),
										corev1.Capability("NET_ADMIN"),
										corev1.Capability("SYS_ADMIN"),
									},
								},
								Privileged: func(b bool) *bool {
									return &b
								}(true),
							},
						},
					},
				},
			},
		},
	}
}

func (c *Controller) ensureVirtualRouterSA(newNS string, virtualRouter *samplev1alpha1.VirtualRouter) error {
	_, err := c.kubeclientset.CoreV1().ServiceAccounts(newNS).Get(context.TODO(), SERVICE_ACCOUNT_NAME, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Error(err)
			return err
		}
		_, err = c.kubeclientset.CoreV1().ServiceAccounts(newNS).Create(context.TODO(), &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SERVICE_ACCOUNT_NAME,
				Namespace: newNS,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(virtualRouter, samplev1alpha1.SchemeGroupVersion.WithKind("VirtualRouter")),
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			klog.Error(err)
			return err
		}
	}
	return nil
}

// ToDo: modify magic string
func (c *Controller) ensureVirtualRouterRole(newNS string, virtualRouter *samplev1alpha1.VirtualRouter) error {
	_, err := c.kubeclientset.RbacV1().Roles(newNS).Get(context.TODO(), ROLE_NAME, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Error(err)
			return err
		}

		_, err = c.kubeclientset.RbacV1().Roles(newNS).Create(context.TODO(), &rbac_v1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ROLE_NAME,
				Namespace: newNS,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(virtualRouter, samplev1alpha1.SchemeGroupVersion.WithKind("VirtualRouter")),
				},
			},
			Rules: []rbac_v1.PolicyRule{
				{
					APIGroups: []string{
						// samplev1alpha1.SchemeGroupVersion.Group,
						virtualrouter.GroupName,
					},
					Resources: []string{
						"natrules", "firewallrules", "loadbalancerrules",
					},
					Verbs: []string{
						"get", "list", "watch", "create", "update", "patch", "delete",
					},
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			klog.Error(err)
			return err
		}
	}
	return nil
}

// ToDo: modify magic string
func (c *Controller) ensureVirtualRouterRoleBinding(newNS string, virtualRouter *samplev1alpha1.VirtualRouter) error {
	_, err := c.kubeclientset.RbacV1().RoleBindings(newNS).Get(context.TODO(), ROLE_BINDING_NAME, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Error(err)
			return err
		}

		_, err = c.kubeclientset.RbacV1().RoleBindings(newNS).Create(context.TODO(), &rbac_v1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ROLE_BINDING_NAME,
				Namespace: newNS,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(virtualRouter, samplev1alpha1.SchemeGroupVersion.WithKind("VirtualRouter")),
				},
			},
			RoleRef: rbac_v1.RoleRef{
				APIGroup: rbac_v1.SchemeGroupVersion.Group,
				Kind:     "Role",
				Name:     ROLE_NAME,
			},
			Subjects: []rbac_v1.Subject{
				{
					Kind: "ServiceAccount",
					Name: SERVICE_ACCOUNT_NAME,
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			klog.Error(err)
			return err
		}
	}
	return nil
}

func (c *Controller) ensureVirtualRouterNamespace(newNS string, virtualRouter *samplev1alpha1.VirtualRouter) error {
	_, err := c.kubeclientset.CoreV1().Namespaces().Get(context.TODO(), newNS, metav1.GetOptions{})
	// RbacV1().ClusterRoles().Get(context.TODO(), clusterRoleName, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			klog.Error(err)
			return err
		}
		_, err := c.kubeclientset.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: newNS,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(virtualRouter, samplev1alpha1.SchemeGroupVersion.WithKind("VirtualRouter")),
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			klog.Error(err)
			return err
		}
	}
	return nil
}

// func (c *Controller) setFinalizer(virtualRouter *samplev1alpha1.VirtualRouter, finalizerStr string) error {
// 	var newVM *samplev1alpha1.VirtualRouter

// 	if !containsString(vm.ObjectMeta.Finalizers, finalizerStr) {
// 		newVM = vm.DeepCopy()
// 		newVM.ObjectMeta.Finalizers = append(newVM.ObjectMeta.Finalizers, finalizerStr)

// 		klog.Infoln("Marking server resource with finalizer")
// 		_, err := c.sampleclientset.SamplecontrollerV1alpha1().VMs(vm.Namespace).Update(newVM)
// 		if err != nil {
// 			klog.Errorln("Marking server resource with finalizer is failed in some reason")
// 			return err
// 		}
// 	}
// 	return nil
// }

// func (c *Controller) deleteFinalizer(virtualRouter *samplev1alpha1.VirtualRouter, finalizerStr string) error {
// 	if containsString(vm.ObjectMeta.Finalizers, finalizerStr) {
// 		deleteCallUrl := vmproviderURL + "/vms/" + vm.Spec.Vmname
// 		req, err := http.NewRequest(http.MethodDelete, deleteCallUrl, nil)
// 		client := &http.Client{}
// 		resp, err := client.Do(req)
// 		if err != nil {
// 			klog.Errorln(err)
// 		}

// 		if resp.StatusCode == http.StatusNoContent {
// 			var newVM *samplev1alpha1.VM
// 			newVM = vm.DeepCopy()
// 			newVM.ObjectMeta.Finalizers = removeString(newVM.ObjectMeta.Finalizers, finalizerStr)
// 			_, err := c.sampleclientset.SamplecontrollerV1alpha1().VMs(vm.Namespace).Update(newVM)
// 			if err != nil {
// 				klog.Errorln("Deleteing finalizer is failed in some reason")
// 				return err
// 			}
// 		} else if resp.StatusCode == http.StatusInternalServerError {
// 			return fmt.Errorf(http.StatusText(resp.StatusCode), nil)
// 		}
// 	}
// 	return nil
// }

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func containString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
