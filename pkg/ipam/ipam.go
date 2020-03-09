package ipam

import (
	"fmt"
	"net"
)

// Manager - handles the addresses for each namespace/vip
var Manager []ipManager

// ipManager defines the mapping to a namespace and address pool
type ipManager struct {
	namespace      string
	cidr           string
	addressManager map[string]bool
	hosts          []string
}

// FindAvailableHost - will look through the cidr and the address Manager and find a free address (if possible)
func FindAvailableHost(namespace, cidr string) (string, error) {

	// Look through namespaces and update one if it exists
	for x := range Manager {
		if Manager[x].namespace == namespace {
			// Check that the address range is the same
			if Manager[x].cidr != cidr {
				// If not rebuild the available hosts
				ah, err := buildHosts(cidr)
				if err != nil {
					return "", err
				}
				Manager[x].hosts = ah
			}
			// TODO - currently we search (incrementally) through the list of hosts
			for y := range Manager[x].hosts {
				// find a host that is marked false (i.e. unused)
				if Manager[x].addressManager[Manager[x].hosts[y]] == false {
					// Mark it to used
					Manager[x].addressManager[Manager[x].hosts[y]] = true
					return Manager[x].hosts[y], nil
				}
			}
		}
	}
	ah, err := buildHosts(cidr)
	if err != nil {
		return "", err
	}
	// If it doesn't exist then it will need adding
	newManager := ipManager{
		namespace:      namespace,
		addressManager: make(map[string]bool),
		hosts:          ah,
		cidr:           cidr,
	}
	Manager = append(Manager, newManager)

	for x := range newManager.hosts {
		if Manager[x].addressManager[newManager.hosts[x]] == false {
			Manager[x].addressManager[newManager.hosts[x]] = true
			return newManager.hosts[x], nil
		}
	}
	return "", fmt.Errorf("No addresses available in [%s] range [%s]", namespace, cidr)

}

// ReleaseAddress - removes the mark on an address
func ReleaseAddress(namespace, address string) error {
	for x := range Manager {
		if Manager[x].namespace == namespace {
			Manager[x].addressManager[address] = false
			return nil

		}
	}
	return fmt.Errorf("Unable to release address [%s] in namespace [%s]", address, namespace)
}

// Builds a list of addresses in the cidr
func buildHosts(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}

	// remove network address and broadcast address
	lenIPs := len(ips)
	switch {
	case lenIPs < 2:
		return ips, nil

	default:
		return ips[1 : len(ips)-1], nil
	}
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
