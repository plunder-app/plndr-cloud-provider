package plndrcp

import (
	"fmt"
	"io"

	"os"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	//ProviderName is the name of the cloud provider
	ProviderName = "plndr"
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

	cfg, err := rest.InClusterConfig()

	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes client config: %s", err.Error())
	}

	cl, err := kubernetes.NewForConfig(cfg)

	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes client: %s", err.Error())
	}

	return &PlunderCloudProvider{
		newLoadBalancer(cl, ns, cm, cidr)}, nil
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

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (p *PlunderCloudProvider) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
func (p *PlunderCloudProvider) Zones() (cloudprovider.Zones, bool) {
	return nil, true
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (p *PlunderCloudProvider) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (p *PlunderCloudProvider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (p *PlunderCloudProvider) ProviderName() string {
	return "plunder"
}

// ScrubDNS provides an opportunity for cloud-provider-specific code to process DNS settings for pods.
func (p *PlunderCloudProvider) ScrubDNS(nameservers, searches []string) (nsOut, srchOut []string) {
	return nil, nil
}

// HasClusterID provides an opportunity for cloud-provider-specific code to process DNS settings for pods.
func (p *PlunderCloudProvider) HasClusterID() bool {
	return false
}

type zones struct{}

func (z zones) GetZone() (cloudprovider.Zone, error) {
	return cloudprovider.Zone{FailureDomain: "FailureDomain1", Region: "Region1"}, nil
}
