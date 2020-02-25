package ipam

import (
	"fmt"
	"net"
)

// Terrible quick IPAM
var addressManager map[string]bool

func init() {
	// initialise the manager list
	addressManager = make(map[string]bool)
}

// FindAvailableHost - will look through the cidr and the address Manager and find a free address (if possible)
func FindAvailableHost(cidr string) (string, error) {
	// Build address range
	ah, err := buildHosts(cidr)
	if err != nil {
		return "", err
	}
	for x := range ah {
		if addressManager[ah[x]] == false {
			addressManager[ah[x]] = true
			return ah[x], nil
		}
	}
	return "", fmt.Errorf("Unable to find address")

}

// ReleaseAddress
func ReleaseAddress(address string) error {
	addressManager[address] = false
	return nil
}

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

// func main() {
// 	host, err := FindAvailableHost("192.168.0.70/30")
// 	if err != nil {
// 		fmt.Printf("%v", err)
// 	}
// 	fmt.Printf("%s\n", host)
// 	host, err = FindAvailableHost("192.168.0.70/30")
// 	if err != nil {
// 		fmt.Printf("%v", err)
// 	}
// 	fmt.Printf("%s\n", host)
// 	addressManager["192.168.0.69"] = false
// 		host, err = FindAvailableHost("192.168.0.70/30")
// 	if err != nil {
// 		fmt.Printf("%v", err)
// 	}
// 	fmt.Printf("%s\n", host)
// }
