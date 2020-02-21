package plndrcp

import (
	"context"
	"encoding/json"

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
	UID         string `json:"uid:`
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
		cm := v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      plb.configMap,
				Namespace: plb.namespace,
			},
		}
		_, err = plb.kubeClient.CoreV1().ConfigMaps(plb.namespace).Create(&cm)
		if err != nil {
			klog.Errorf("%v", err)
		}
	}
	var svc plndrServices

	if cm.Data == nil {
		cm.Data = map[string]string{}
		cm.Data["services"] = svc.updateServices(vip, service.Name, string(service.UID))

	} else {
		b := cm.Data["services"]
		json.Unmarshal([]byte(b), &svc)
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
