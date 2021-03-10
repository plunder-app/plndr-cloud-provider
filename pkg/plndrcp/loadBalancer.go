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
		cloudConfigMap: cm,
	}
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

	// Find the services configuration in the configMap
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
		err = ipam.ReleaseAddress(service.Namespace, service.Spec.LoadBalancerIP)
		if err != nil {
			klog.Errorln(err)
		}
	}
	// Update the configMap
	_, err = plb.UpdateConfigMap(cm, updatedSvc)
	return err
}

func (plb *plndrLoadBalancerManager) syncLoadBalancer(service *v1.Service) (*v1.LoadBalancerStatus, error) {

	// Get the clound controller configuration map
	controllerCM, err := plb.GetConfigMap(PlunderCloudConfig, "kube-system")
	if err != nil {
		klog.Errorf("Unable to retrieve kube-vip ipam config from configMap [%s] in kube-system", PlunderClientConfig)
		// TODO - determine best course of action, create one if it doesn't exist
		controllerCM, err = plb.CreateConfigMap(PlunderCloudConfig, "kube-system")
		if err != nil {
			return nil, err
		}
	}

	// Retrieve the kube-vip configuration map
	namespaceCM, err := plb.GetConfigMap(PlunderClientConfig, service.Namespace)
	if err != nil {
		klog.Errorf("Unable to retrieve kube-vip service cache from configMap [%s] in [%s]", PlunderClientConfig, service.Namespace)
		// TODO - determine best course of action
		namespaceCM, err = plb.CreateConfigMap(PlunderClientConfig, service.Namespace)
		if err != nil {
			return nil, err
		}
	}

	// This function reconciles the load balancer state
	klog.Infof("syncing service '%s' (%s)", service.Name, service.UID)

	// Find the services configuraiton in the configMap
	svc, err := plb.GetServices(namespaceCM)
	if err != nil {
		klog.Errorf("Unable to retrieve services from configMap [%s], [%s]", PlunderClientConfig, err.Error())

		// TODO best course of action, currently we create a new services config
		svc = &plndrServices{}
	}

	// Check for existing configuration

	existing := svc.findService(string(service.UID))
	if existing != nil {
		klog.Infof("found existing service '%s' (%s) with vip %s", service.Name, service.UID, existing.Vip)

		// If this is 0.0.0.0 then it's a DHCP lease and we need to return that not the 0.0.0.0
		if existing.Vip == "0.0.0.0" {
			return &service.Status.LoadBalancer, nil
		}

		//
		return &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{
				{
					IP: existing.Vip,
				},
			},
		}, nil
	}

	var vip string

	if service.Spec.LoadBalancerIP != "" {
		// An IP address is specified, we need to validate it and then allocate it
		vip = service.Spec.LoadBalancerIP
	} else {
		vip, err = discoverAddress(controllerCM, service.Namespace, plb.cloudConfigMap)
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

	service.Spec.LoadBalancerIP = vip

	updatedService, err := plb.kubeClient.CoreV1().Services(service.Namespace).Update(service)
	klog.Infof("Updating service [%s], with load balancer address [%s]", updatedService.Name, updatedService.Spec.LoadBalancerIP)
	if err != nil {
		// release the address internally as we failed to update service
		err = ipam.ReleaseAddress(service.Namespace, vip)
		if err != nil {
			klog.Errorln(err)
		}
		return nil, fmt.Errorf("Error updating Service Spec [%s] : %v", service.Name, err)
	}

	svc.addService(newSvc)

	namespaceCM, err = plb.UpdateConfigMap(namespaceCM, svc)
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

func discoverAddress(cm *v1.ConfigMap, namespace, configMapName string) (vip string, err error) {
	var cidr, ipRange string
	var ok bool

	// Find Cidr
	cidrKey := fmt.Sprintf("cidr-%s", namespace)
	// Lookup current namespace
	if cidr, ok = cm.Data[cidrKey]; !ok {
		klog.Info(fmt.Errorf("No cidr config for namespace [%s] exists in key [%s] configmap [%s]", namespace, cidrKey, configMapName))
		// Lookup global cidr configmap data
		if cidr, ok = cm.Data["cidr-global"]; !ok {
			klog.Info(fmt.Errorf("No global cidr config exists [cidr-global]"))
		} else {
			klog.Infof("Taking address from [cidr-global] pool")
		}
	} else {
		klog.Infof("Taking address from [%s] pool", cidrKey)
	}
	if ok {
		vip, err = ipam.FindAvailableHostFromCidr(namespace, cidr)
		if err != nil {
			return "", err
		}
		return
	}

	// Find Range
	rangeKey := fmt.Sprintf("range-%s", namespace)
	// Lookup current namespace
	if ipRange, ok = cm.Data[rangeKey]; !ok {
		klog.Info(fmt.Errorf("No range config for namespace [%s] exists in key [%s] configmap [%s]", namespace, rangeKey, configMapName))
		// Lookup global range configmap data
		if ipRange, ok = cm.Data["range-global"]; !ok {
			klog.Info(fmt.Errorf("No global range config exists [range-global]"))
		} else {
			klog.Infof("Taking address from [range-global] pool")
		}
	} else {
		klog.Infof("Taking address from [%s] pool", rangeKey)
	}
	if ok {
		vip, err = ipam.FindAvailableHostFromRange(namespace, ipRange)
		if err != nil {
			return vip, err
		}
		return
	}
	return "", fmt.Errorf("No IP address ranges could be found either range-global or range-<namespace>")
}
