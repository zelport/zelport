package remote_http

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func ParseRemoteURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Opaque != "" {
		return nil, errors.New("Target URL must be an absolute http:// or https:// URL")
	}
	if u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") {
		return nil, errors.New("Target URL may include a base path, but not credentials, a query, or a fragment")
	}
	host := u.Hostname()
	if host == "" || strings.ContainsAny(host, "\\%") {
		return nil, errors.New("Target URL must contain a valid hostname or IP address")
	}
	if net.ParseIP(host) == nil {
		for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return nil, errors.New("invalid target hostname")
			}
			for _, c := range label {
				if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
					return nil, errors.New("use an ASCII hostname or IP address in Target URL")
				}
			}
		}
		if len(host) > 253 {
			return nil, errors.New("target hostname is too long")
		}
	}
	if strings.HasSuffix(u.Host, ":") {
		return nil, errors.New("target port is empty")
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return nil, errors.New("target port must be between 1 and 65535")
		}
	}
	return u, nil
}
