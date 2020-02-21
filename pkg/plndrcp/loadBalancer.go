package plndrcp

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type plndrServices struct {
	Services []services `json:"services"`
}

type services struct {
	Vip         string `json:"vip"`
	UID         string `json:"uid"`
	ServiceName string `json:"serviceName"`
}

//PlndrLoadBalancer -
type plndrLoadBalancerManager struct {
	kubeClient  *kubernetes.Clientset
	namespace   string
	configMap   string
	serviceCidr string
}

func newLoadBalancer(kubeClient *kubernetes.Clientset, ns, cm, serviceCidr string) cloudprovider.LoadBalancer {
	return &plndrLoadBalancerManager{
		kubeClient:  kubeClient,
		namespace:   ns,
		configMap:   cm,
		serviceCidr: serviceCidr}
}

func (plb *plndrLoadBalancerManager) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (lbs *v1.LoadBalancerStatus, err error) {
	return plb.syncLoadBalancer(service)
}
func (plb *plndrLoadBalancerManager) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (err error) {
	_, err = plb.syncLoadBalancer(service)
	return err
}

func (plb *plndrLoadBalancerManager) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.Infof("Deleting service '%s' (%s)", service.Name, service.UID)

	return plb.deleteLoadBalancer(service)
}

func (plb *plndrLoadBalancerManager) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	cm, err := plb.kubeClient.CoreV1().ConfigMaps(plb.namespace).Get(plb.configMap, metav1.GetOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("Unable to find configMap [%s]", plb.configMap)
	}
	var svcs plndrServices
	d := cm.Data["services"]
	json.Unmarshal([]byte(d), &svcs)

	for x := range svcs.Services {
		if svcs.Services[x].UID == string(service.UID) {
			return &v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: svcs.Services[x].Vip,
					},
				},
			}, true, nil
		}
	}

	return nil, false, fmt.Errorf("Unable to find service [%s]", service.Name)
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
	klog.Infof("syncing (deleting) service '%s' (%s)", service.Name, service.UID)
	//	return nil, fmt.Errorf("BOOM, no kube-vip for you ..")

	cm, err := plb.kubeClient.CoreV1().ConfigMaps(plb.namespace).Get(plb.configMap, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Unable to find configMap [%s]", plb.configMap)
	}
	var oldServices, newServices plndrServices
	d := cm.Data["services"]
	json.Unmarshal([]byte(d), &oldServices)

	var found bool
	for x := range oldServices.Services {
		if oldServices.Services[x].UID != string(service.UID) {
			newServices.Services = append(newServices.Services, oldServices.Services[x])
		} else {
			found = true
		}
	}

	b, _ := json.Marshal(newServices)

	cm.Data["services"] = string(b)
	_, err = plb.kubeClient.CoreV1().ConfigMaps(plb.namespace).Update(cm)

	if err != nil {
		klog.Errorf("%v", err)
	}

	if found != true {
		return fmt.Errorf("Unable to find service [%s] in configMap [%s]", service.Name, plb.configMap)
	}

	return nil
}

func (plb *plndrLoadBalancerManager) syncLoadBalancer(service *v1.Service) (*v1.LoadBalancerStatus, error) {
	var vip string
	vip = "192.168.0.76"
	// This function reconciles the load balancer state
	klog.Infof("syncing service '%s' (%s) with vip: %s", service.Name, service.UID, vip)
	//	return nil, fmt.Errorf("BOOM, no kube-vip for you ..")

	cm, err := plb.kubeClient.CoreV1().ConfigMaps(plb.namespace).Get(plb.configMap, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Can't find config Map %s, creating new Map", plb.configMap)
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      plb.configMap,
				Namespace: plb.namespace,
			},
		}
		cm.Data = map[string]string{}
		cm, err = plb.kubeClient.CoreV1().ConfigMaps(plb.namespace).Create(cm)
		if err != nil {
			klog.Errorf("%v", err)
		}
	}
	var svc plndrServices

	if cm.Data == nil {
		cm.Data = map[string]string{}

	}
	b := cm.Data["services"]
	json.Unmarshal([]byte(b), &svc)

	var found bool
	for x := range svc.Services {
		if svc.Services[x].UID == string(service.UID) {
			svc.Services[x].Vip = vip
			svc.Services[x].ServiceName = cm.Name
			found = true
		}
	}

	if found == true {
		b, _ := json.Marshal(svc)
		cm.Data["services"] = string(b)
	} else {
		cm.Data["services"] = svc.updateServices(vip, service.Name, string(service.UID))
	}

	if cm.Annotations == nil {
		cm.Annotations = map[string]string{}
	}

	_, err = plb.kubeClient.CoreV1().ConfigMaps(plb.namespace).Update(cm)

	if err != nil {
		klog.Errorf("%v", err)
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: vip,
			},
		},
	}, nil
}

func (s *plndrServices) updateServices(vip, name, uid string) string {
	newsvc := services{
		Vip:         vip,
		UID:         uid,
		ServiceName: name,
	}
	s.Services = append(s.Services, newsvc)
	b, _ := json.Marshal(s)
	return string(b)
}

// func (plb *plndrLoadBalancerManager) getConfigMap() (*v1.ConfigMap, error) {

// 	return nil, nil
// }
