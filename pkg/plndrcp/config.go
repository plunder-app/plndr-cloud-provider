package plndrcp

import (
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Services functions - once the service data is taken from teh configMap, these functions will interact with the data

func (s *plndrServices) addService(newSvc services) {
	s.Services = append(s.Services, newSvc)
}

func (s *plndrServices) delServiceFromUID(UID string) *plndrServices {
	// New Services list
	updatedServices := &plndrServices{}
	// Add all [BUT] the removed service
	for x := range s.Services {
		if s.Services[x].UID != UID {
			updatedServices.Services = append(updatedServices.Services, s.Services[x])
		}
	}
	// Return the updated service list (without the mentioned service)
	return updatedServices
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

// ConfigMap functions - these wrap all interactions with the kubernetes configmaps

func (plb *plndrLoadBalancerManager) GetServices(cm *v1.ConfigMap) (svcs *plndrServices, err error) {
	// Attempt to retrieve the config map
	b := cm.Data[PlunderServicesKey]
	fmt.Printf("%v\n", cm.Data)
	// Unmarshall raw data into struct
	err = json.Unmarshal([]byte(b), &svcs)
	return
}

func (plb *plndrLoadBalancerManager) GetConfigMap() (*v1.ConfigMap, error) {
	// Attempt to retrieve the config map
	return plb.kubeClient.CoreV1().ConfigMaps(plb.namespace).Get(plb.configMap, metav1.GetOptions{})
}

func (plb *plndrLoadBalancerManager) CreateConfigMap() (*v1.ConfigMap, error) {
	// Create new configuration map in the correct namespace
	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      plb.configMap,
			Namespace: plb.namespace,
		},
	}
	// Return results of configMap create
	return plb.kubeClient.CoreV1().ConfigMaps(plb.namespace).Create(&cm)
}

func (plb *plndrLoadBalancerManager) UpdateConfigMap(cm *v1.ConfigMap, s *plndrServices) (*v1.ConfigMap, error) {
	// Create new configuration map in the correct namespace

	// If the cm.Data / cm.Annotations haven't been initialised
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	if cm.Annotations == nil {
		cm.Annotations = map[string]string{}
		cm.Annotations["provider"] = ProviderName
	}

	// Set ConfigMap data
	b, _ := json.Marshal(s)
	cm.Data[PlunderServicesKey] = string(b)

	// TODO - in this first release the CIDR will be static
	cm.Data["cidr"] = plb.serviceCidr

	// Return results of configMap create
	return plb.kubeClient.CoreV1().ConfigMaps(plb.namespace).Update(cm)
}
