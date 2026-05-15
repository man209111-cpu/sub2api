// Package ipgeo provides best-effort IP geolocation lookup for audit display.
package ipgeo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type Info struct {
	IPAddress   string
	Country     string
	CountryCode string
	Region      string
	City        string
	Location    string
}

func Lookup(ctx context.Context, rawIP string) (*Info, error) {
	ip := strings.TrimSpace(rawIP)
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, fmt.Errorf("invalid ip")
	}
	if isLocalIP(parsed) {
		return &Info{IPAddress: ip}, nil
	}

	endpoint := "http://ip-api.com/json/" + url.PathEscape(ip) + "?lang=zh-CN&fields=status,message,country,countryCode,regionName,region,city,query"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lookup failed: status %d", resp.StatusCode)
	}

	var body struct {
		Status      string `json:"status"`
		Message     string `json:"message"`
		Query       string `json:"query"`
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		Region      string `json:"region"`
		RegionName  string `json:"regionName"`
		City        string `json:"city"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if strings.ToLower(body.Status) != "success" {
		if body.Message == "" {
			body.Message = "ip lookup failed"
		}
		return nil, errors.New(body.Message)
	}

	region := strings.TrimSpace(body.RegionName)
	if region == "" {
		region = strings.TrimSpace(body.Region)
	}
	info := &Info{
		IPAddress:   firstNonEmpty(body.Query, ip),
		Country:     strings.TrimSpace(body.Country),
		CountryCode: strings.TrimSpace(body.CountryCode),
		Region:      region,
		City:        strings.TrimSpace(body.City),
	}
	info.Location = formatLocation(info.Country, info.Region, info.City)
	return info, nil
}

func isLocalIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func formatLocation(parts ...string) string {
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key := strings.ToLower(part)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, part)
	}
	return strings.Join(out, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
