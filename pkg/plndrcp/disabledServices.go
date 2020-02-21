package plndrcp

import cloudprovider "k8s.io/cloud-provider"

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
