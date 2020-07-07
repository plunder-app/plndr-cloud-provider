package plndrcp

import (
	"context"
	"fmt"

	"github.com/plunder-app/plndr-cloud-provider/pkg/ipam"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

type plndrServices struct {
	Services []services `json:"services"`
}

type services struct {
	Vip         string `json:"vip"`
	Port        int    `json:"port"`
	Type        string `json:"type"`
	UID         string `json:"uid"`
	ServiceName string `json:"serviceName"`
}

//PlndrLoadBalancer -
type plndrLoadBalancerManager struct {
	kubeClient     *kubernetes.Clientset
	nameSpace      string
	cloudConfigMap string
}

func newLoadBalancer(kubeClient *kubernetes.Clientset, ns, cm, serviceCidr string) cloudprovider.LoadBalancer {
	return &plndrLoadBalancerManager{
		kubeClient:     kubeClient,
		nameSpace:      ns,
		cloudConfigMap: cm}
}

func (plb *plndrLoadBalancerManager) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (lbs *v1.LoadBalancerStatus, err error) {
	return plb.syncLoadBalancer(service)
}
func (plb *plndrLoadBalancerManager) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (err error) {
	_, err = plb.syncLoadBalancer(service)
	return err
}

func (plb *plndrLoadBalancerManager) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	return plb.deleteLoadBalancer(service)
}

func (plb *plndrLoadBalancerManager) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {

	// Retrieve the kube-vip configuration from it's namespace
	cm, err := plb.GetConfigMap(PlunderClientConfig, service.Namespace)
	if err != nil {
		return nil, true, nil
	}

	// Find the services configuraiton in the configMap
	svc, err := plb.GetServices(cm)
	if err != nil {
		return nil, false, err
	}

	for x := range svc.Services {
		if svc.Services[x].UID == string(service.UID) {
			return &v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: svc.Services[x].Vip,
					},
				},
			}, true, nil
		}
	}
	return nil, false, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (plb *plndrLoadBalancerManager) GetLoadBalancerName(_ context.Context, clusterName string, service *v1.Service) string {
	return getDefaultLoadBalancerName(service)
}

func getDefaultLoadBalancerName(service *v1.Service) string {
	return cloudprovider.DefaultLoadBalancerName(service)
}
func (plb *plndrLoadBalancerManager) deleteLoadBalancer(service *v1.Service) error {
	klog.Infof("deleting service '%s' (%s)", service.Name, service.UID)

	// Get the kube-vip (client) configuration from it's namespace
	cm, err := plb.GetConfigMap(PlunderClientConfig, service.Namespace)
	if err != nil {
		klog.Errorf("The configMap [%s] doensn't exist", PlunderClientConfig)
		return nil
	}
	// Find the services configuraiton in the configMap
	svc, err := plb.GetServices(cm)
	if err != nil {
		klog.Errorf("The service [%s] in configMap [%s] doensn't exist", service.Name, PlunderClientConfig)
		return nil
	}

	// Update the services configuration, by removing the  service
	updatedSvc := svc.delServiceFromUID(string(service.UID))
	if len(service.Status.LoadBalancer.Ingress) != 0 {
		ipam.ReleaseAddress(service.Namespace, service.Status.LoadBalancer.Ingress[0].IP)
	}
	// Update the configMap
	_, err = plb.UpdateConfigMap(cm, updatedSvc)
	return err
}

func (plb *plndrLoadBalancerManager) syncLoadBalancer(service *v1.Service) (*v1.LoadBalancerStatus, error) {

	// Get the clound controller configuration map
	cm, err := plb.GetConfigMap(PlunderCloudConfig, "kube-system")
	if err != nil {
		// TODO - determine best course of action, create one if it doesn't exist
		cm, err = plb.CreateConfigMap(PlunderCloudConfig, "kube-system")
		if err != nil {
			return nil, err
		}
	}

	// This function reconciles the load balancer state
	klog.Infof("syncing service '%s' (%s) with", service.Name, service.UID)

	// Find the services configuraiton in the configMap
	svc, err := plb.GetServices(cm)
	if err != nil {
		klog.Errorf("Unable to retrieve services from configMap [%s]", PlunderClientConfig)

		// TODO best course of action, currently we create a new services config
		svc = &plndrServices{}
	}

	// Check for existing configuration

	existing := svc.findService(string(service.UID))
	if existing != nil {
		klog.Infof("found existing service '%s' (%s) with vip %s", service.Name, service.UID, existing.Vip)
		return &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{
				{
					IP: existing.Vip,
				},
			},
		}, nil
	}

	var vip, cidrRange string
	var ok bool
	// Build cidr key for this service
	cidrKey := fmt.Sprintf("cidr-%s", service.Namespace)
	if cidrRange, ok = cm.Data[cidrKey]; !ok {
		return nil, fmt.Errorf("No cidr configuration for namespace [%s] exists in key [%s] configmap [%s]", service.Namespace, cidrKey, plb.cloudConfigMap)

	}

	// Check if we're not explicitly specifying an address to use, if not then use iPAM to find an address
	if service.Spec.LoadBalancerIP == "" {
		vip, err = ipam.FindAvailableHost(service.Namespace, cidrRange)
		if err != nil {
			return nil, err
		}
	} else {
		// An IP address is specified, we need to validate it and then allocate it
		vip = service.Spec.LoadBalancerIP
	}

	// Retrieve the kube-vip configuration map
	cm, err = plb.GetConfigMap(PlunderClientConfig, service.Namespace)
	if err != nil {
		// TODO - determine best course of action
		cm, err = plb.CreateConfigMap(PlunderClientConfig, service.Namespace)
		if err != nil {
			return nil, err
		}
	}

	// TODO - manage more than one set of ports
	newSvc := services{
		ServiceName: service.Name,
		UID:         string(service.UID),
		Type:        string(service.Spec.Ports[0].Protocol),
		Vip:         vip,
		Port:        int(service.Spec.Ports[0].Port),
	}

	svc.addService(newSvc)

	cm, err = plb.UpdateConfigMap(cm, svc)
	if err != nil {
		return nil, err
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: vip,
			},
		},
	}, nil
}
