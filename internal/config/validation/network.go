package validation

import (
	"net"
	"net/url"
	"regexp"
)

func IsValidUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	if value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}

	validChars := "0123456789abcdefABCDEF-"
	for _, char := range value {
		found := false
		for _, validChar := range validChars {
			if char == validChar {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func IsValidURL(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsedURL.Scheme == "http" || parsedURL.Scheme == "https"
}

func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func IsValidAWSRegion(region string) bool {
	regionPattern := regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d+$`)
	return regionPattern.MatchString(region)
}

func IsValidCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

func SubnetsOverlap(cidr1, cidr2 string) bool {
	_, net1, err1 := net.ParseCIDR(cidr1)
	_, net2, err2 := net.ParseCIDR(cidr2)
	if err1 != nil || err2 != nil {
		return false
	}
	return net1.Contains(net2.IP) || net2.Contains(net1.IP)
}
