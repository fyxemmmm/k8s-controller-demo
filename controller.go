package main

import (
	"fmt"
	crdv1beta1 "github.com/cnych/controller-demo/pkg/apis/stable/v1beta1"
	informer "github.com/cnych/controller-demo/pkg/client/informers/externalversions/stable/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"time"
)

type Controller struct {
	informer informer.CronTabInformer
	queue    workqueue.RateLimitingInterface
}

func NewController(informer informer.CronTabInformer) *Controller {
	controller := &Controller{
		informer: informer,
		queue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "crontab-controller"),
	}
	klog.Info("setting up crontab controller")
	// 注册事件监听函数
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.onAdd,
		UpdateFunc: controller.onUpdate,
		DeleteFunc: controller.onDelete,
	})
	return controller
}

func (c *Controller) onAdd(obj interface{}) {
	// object 转换成key
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.AddRateLimited(key)
}

func (c *Controller) onUpdate(old, new interface{}) {
	oldObj := old.(*crdv1beta1.CronTab)
	newObj := new.(*crdv1beta1.CronTab)
	// 比较资源版本是否一致
	if oldObj.ResourceVersion == newObj.ResourceVersion {
		return
	}
	c.onAdd(newObj)
}

func (c *Controller) onDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	c.queue.AddRateLimited(key)
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	// 关闭队列
	defer c.queue.ShutDown()

	// 启动控制器
	klog.Info("starting crontab controller")

	// 缓存没同步完成就不处理队列中的数据
	if !cache.WaitForCacheSync(stopCh, c.informer.Informer().HasSynced) {
		return fmt.Errorf("time out waiting for cache sync")
	}

	klog.Info("informer cache sync completed")

	// 启动worker处理
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("stopping crontab controller")
	return nil
}

// 处理元素
func (c *Controller) runWorker() {
	fmt.Println("in..")
	for c.processNextItem() {

	}
}

// 实现业务逻辑
func (c *Controller) processNextItem() bool {
	// 从workqueue里取出一个元素
	obj, shutdown := c.queue.Get()
	if shutdown { // 队列关闭了
		return false
	}

	// 根据 key 去处理我们的业务逻辑
	err := func(obj interface{}) error {
		// 告诉队列我们已经处理了该key
		defer c.queue.Done(obj)

		var ok bool
		var key string
		if key, ok = obj.(string); !ok {
			c.queue.Forget(obj) // 拿的key应该是字符串, 工作队列中的字符串
			return fmt.Errorf("expect string in workqueue, but get %#v", obj)
		}
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("sync error: %v", err)
		}
		c.queue.Forget(key)
		klog.Info("successfully synced %s", key)
		return nil
	}(obj) // 本质是个key
	if err != nil {
		runtime.HandleError(err)
	}
	return true
}

// key -> crontab -> indexer
func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	// 获取 crontab
	crontab, err := c.informer.Lister().CronTabs(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			// 缓存中没有, 对于的crontab对象已经被删除了
			klog.Warningf("crontab deleted: %s/%s", namespace, name)
			return nil
		}
		return err
	}

	klog.Info("crontab try to process: %#v", crontab)
	return nil
}
