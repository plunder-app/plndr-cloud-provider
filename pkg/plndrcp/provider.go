package plndrcp

import (
	"fmt"
	"io"
	"path/filepath"

	"os"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	cloudprovider "k8s.io/cloud-provider"
)

// OutSideCluster allows the controller to be started using a local kubeConfig for testing
var OutSideCluster bool

const (
	//ProviderName is the name of the cloud provider
	ProviderName = "plndr"

	//PlunderCloudConfig is the default name of the load balancer config Map
	PlunderCloudConfig = "plndr"

	//PlunderClientConfig is the default name of the load balancer config Map
	PlunderClientConfig = "plndr"

	//PlunderServicesKey is the key in the ConfigMap that has the services configuration
	PlunderServicesKey = "plndr-services"
)

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, newPlunderCloudProvider)
}

// PlunderCloudProvider - contains all of the interfaces for the cloud provider
type PlunderCloudProvider struct {
	lb cloudprovider.LoadBalancer
}

var _ cloudprovider.Interface = &PlunderCloudProvider{}

func newPlunderCloudProvider(io.Reader) (cloudprovider.Interface, error) {
	ns := os.Getenv("PLNDR_NAMESPACE")
	cm := os.Getenv("PLNDR_CONFIG_MAP")
	cidr := os.Getenv("PLNDR_SERVICE_CIDR")

	if cm == "" {
		cm = PlunderCloudConfig
	}

	if ns == "" {
		ns = "default"
	}

	var cl *kubernetes.Clientset
	if OutSideCluster == false {
		// This will attempt to load the configuration when running within a POD
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("error creating kubernetes client config: %s", err.Error())
		}
		cl, err = kubernetes.NewForConfig(cfg)

		if err != nil {
			return nil, fmt.Errorf("error creating kubernetes client: %s", err.Error())
		}
		// use the current context in kubeconfig
	} else {
		config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			panic(err.Error())
		}
		cl, err = kubernetes.NewForConfig(config)

		if err != nil {
			return nil, fmt.Errorf("error creating kubernetes client: %s", err.Error())
		}
	}
	return &PlunderCloudProvider{
		lb: newLoadBalancer(cl, ns, cm, cidr),
	}, nil
}

// Initialize - starts the clound-provider controller
func (p *PlunderCloudProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	clientset := clientBuilder.ClientOrDie("do-shared-informers")
	sharedInformer := informers.NewSharedInformerFactory(clientset, 0)

	//res := NewResourcesController(c.resources, sharedInformer.Core().V1().Services(), clientset)

	sharedInformer.Start(nil)
	sharedInformer.WaitForCacheSync(nil)
	//go res.Run(stop)
	//go c.serveDebug(stop)
}

// LoadBalancer returns a loadbalancer interface. Also returns true if the interface is supported, false otherwise.
func (p *PlunderCloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return p.lb, true
}

// ProviderName returns the cloud provider ID.
func (p *PlunderCloudProvider) ProviderName() string {
	return ProviderName
}
