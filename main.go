package main

import (
	"flag"
	"fmt"
	v12 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"path/filepath"
	"time"
)

type Controller struct {
	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller
}

func NewController(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller) *Controller {
	return &Controller{
		informer: informer,
		indexer:  indexer,
		queue:    queue,
	}
}
func (c *Controller) Run(threadiness int, stopper chan struct{}) {

	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	klog.Info("Starting Deployment Controller")
	go c.informer.Run(stopper)

	if !cache.WaitForCacheSync(stopper, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timeout waiting for caches to sync"))
		return
	}
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopper)
	}
	<-stopper
	klog.Info("Stopping Deployment controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}
func (c *Controller) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)
	err := c.notify(key.(string))
	c.handleError(err, key)
	return true
}
func (c *Controller) notify(key string) error {
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}
	if !exists {
		c.objectDeleted(key)
	} else {
		c.objectCreated(obj)
	}
	return nil
}
func (c *Controller) handleError(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}
	if c.queue.NumRequeues(key) < 5 {
		klog.Infof("Error syncing Deployment %v: %v", key, err)
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)
	runtime.HandleError(err)
	klog.Infof("Dropping Deployment %q out of the queue: %v", key, err)
}
func (c *Controller) objectCreated(obj interface{}){
	fmt.Printf("%s Created\n", obj.(*v12.Deployment).GetName())
}
func (c *Controller) objectDeleted(key string){
	fmt.Printf("%s Deleted\n", key)
}
func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeConfig file")
	}
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Println(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	deployListWatcher := cache.NewListWatchFromClient(clientset.AppsV1().RESTClient(), "deployments", v1.NamespaceDefault, fields.Everything())
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	indexer, informer := cache.NewIndexerInformer(deployListWatcher, &v12.Deployment{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{})
	controller := NewController(queue, indexer, informer)
	stopper := make(chan struct{})
	defer close(stopper)
	go controller.Run(1, stopper)
	//wait forever
	select {}
}
