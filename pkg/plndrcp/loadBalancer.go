package plndrcp

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
)

//PlndrLoadBalancer -
type plndrLoadBalancer struct {
	kubeClient  *kubernetes.Clientset
	namespace   string
	name        string
	serviceCidr string
}

func newLoadBalancer(kubeClient *kubernetes.Clientset, ns, name, serviceCidr string) cloudprovider.LoadBalancer {
	return &plndrLoadBalancer{
		kubeClient:  kubeClient,
		namespace:   ns,
		name:        name,
		serviceCidr: serviceCidr}
}

func (plb *plndrLoadBalancer) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (lbs *v1.LoadBalancerStatus, err error) {
	return plb.syncLoadBalancer(service)
}
func (plb *plndrLoadBalancer) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (err error) {
	return err
}

func (plb *plndrLoadBalancer) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	return plb.deleteLoadBalancer(service)
}

func (plb *plndrLoadBalancer) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	return nil, false, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (plb *plndrLoadBalancer) GetLoadBalancerName(_ context.Context, clusterName string, service *v1.Service) string {
	return getDefaultLoadBalancerName(service)
}

func getDefaultLoadBalancerName(service *v1.Service) string {
	return cloudprovider.DefaultLoadBalancerName(service)
}
func (plb *plndrLoadBalancer) deleteLoadBalancer(service *v1.Service) error {

	return nil
}

func (plb *plndrLoadBalancer) syncLoadBalancer(service *v1.Service) (*v1.LoadBalancerStatus, error) {

	return nil, nil
}

func (plb *plndrLoadBalancer) getConfigMap() (*v1.ConfigMap, error) {

	return nil, nil
}
