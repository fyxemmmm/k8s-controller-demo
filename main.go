package main


import (
	"flag"
	"github.com/cnych/controller-demo/pkg/client/clientset/versioned"
	"github.com/cnych/controller-demo/pkg/client/informers/externalversions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func initClient() (*kubernetes.Clientset, *rest.Config, error) {
	var err error
	var config *rest.Config
	// inCluster（Pod）、KubeConfig（kubectl）
	var kubeconfig *string

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(可选) kubeconfig 文件的绝对路径")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "kubeconfig 文件的绝对路径")
	}
	flag.Parse()

	// 首先使用 inCluster 模式(需要去配置对应的 RBAC 权限，默认的sa是default->是没有获取deployments的List权限)
	if config, err = rest.InClusterConfig(); err != nil {
		// 使用 KubeConfig 文件创建集群配置 Config 对象
		if config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig); err != nil {
			panic(err.Error())
		}
	}

	clientset, err :=  kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return clientset, config, nil
}

func setupSignalHandler() (stopCh <-chan struct{}) {
	stop := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, []os.Signal{os.Interrupt, syscall.SIGTERM}...)
	go func() {
		<- c
		close(stop)
		<- c
		os.Exit(1)
	}()
	return stop
}


func main() {
	flag.Parse()
	_, config, err := initClient()
	if err != nil {
		klog.Fatalf("error init k8s client: %s", err.Error())
	}

	crontabClientset, err := versioned.NewForConfig(config)
	if err != nil {
		klog.Fatalf("error init k8s crontab client: %s", err.Error())
	}

	stopCh := setupSignalHandler()

	// 定义一个自定义控制器的informer
	sharedinformerFactory := externalversions.NewSharedInformerFactory(crontabClientset, time.Second * 30)

	controller := NewController(
		sharedinformerFactory.Stable().V1beta1().CronTabs(),
	)

	// 启动informer, 执行list & watch
	go sharedinformerFactory.Start(stopCh)

	// 启动控制器的控制循环
	if err := controller.Run(1, stopCh); err != nil {
		klog.Fatalf("error running crontab controller err %s", err.Error())
	}
}



