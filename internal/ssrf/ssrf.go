package ssrf

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// ValidateURL rejects URLs that could target internal infrastructure.
// Pure function (does DNS via net.LookupIP); unit-testable.
func ValidateURL(raw string) error {
	if len(raw) == 0 {
		return errors.New("url is required")
	}
	if len(raw) > 2048 {
		return errors.New("url too long (max 2048)")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("only http and https schemes allowed")
	}
	if u.User != nil {
		return errors.New("userinfo in url is not allowed")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("url host is required")
	}
	lower := strings.ToLower(host)
	// Hostname blocklist
	if lower == "localhost" {
		return errors.New("blocked host: localhost")
	}
	for _, suf := range []string{".localhost", ".internal", ".local", ".lan", ".home", ".corp"} {
		if strings.HasSuffix(lower, suf) {
			return fmt.Errorf("blocked internal hostname suffix %q", suf)
		}
	}
	if lower == "metadata.google.internal" || lower == "metadata.google" || strings.Contains(lower, "169.254.169.254") {
		return errors.New("blocked cloud metadata host")
	}
	// Numeric IP bypass: try to interpret host as IP in decimal/octal/hex forms.
	if ip, ok := parseNumericHost(host); ok {
		if isBlockedIP(ip) {
			return fmt.Errorf("blocked ip address: %s", ip.String())
		}
		return nil
	}
	// IP literal
	if ip, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("blocked ip address: %s", ip.String())
		}
		return nil
	}
	// DNS: resolve and check every address
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return errors.New("dns resolution failed or returned no addresses")
	}
	for _, nip := range ips {
		addr, perr := netip.ParseAddr(nip.String())
		if perr != nil {
			return fmt.Errorf("unparseable resolved ip: %s", nip.String())
		}
		if isBlockedIP(addr) {
			return fmt.Errorf("blocked resolved ip %s for host %s", addr.String(), host)
		}
	}
	return nil
}

func isBlockedIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || !ip.IsValid() {
		return true
	}
	if ip.Is4() {
		b := ip.As4()
		// 0.0.0.0/8
		if b[0] == 0 {
			return true
		}
		// 127/8
		if b[0] == 127 {
			return true
		}
		// 10/8
		if b[0] == 10 {
			return true
		}
		// 172.16/12
		if b[0] == 172 && b[1] >= 16 && b[1] <= 31 {
			return true
		}
		// 192.168/16
		if b[0] == 192 && b[1] == 168 {
			return true
		}
		// 169.254/16 link-local / cloud metadata
		if b[0] == 169 && b[1] == 254 {
			return true
		}
		// 100.64/10 CGNAT
		if b[0] == 100 && b[1] >= 64 && b[1] <= 127 {
			return true
		}
		// 192.0.2/24, 198.51.100/24, 203.0.113/24 (docs), 192.88.99/24, 224/4 multicast, 240/4 reserved
		if b[0] >= 224 {
			return true
		}
		return false
	}
	// IPv6: ::1 handled by IsLoopback above; block fc00::/7, fe80::/10
	b := ip.As16()
	if b[0]&0xfe == 0xfc { // fc00::/7
		return true
	}
	if b[0] == 0xfe && (b[1]&0xc0) == 0x80 { // fe80::/10
		return true
	}
	if ip.IsUnspecified() {
		return true
	}
	return false
}

// parseNumericHost handles decimal/octal/hex bypass forms like
// "2130706433" (127.0.0.1), "0177.0.0.1", "0x7f.0.0.1".
func parseNumericHost(host string) (netip.Addr, bool) {
	host = strings.Trim(host, "[]")
	if !strings.Contains(host, ".") {
		// single integer form
		n, err := strconv.ParseUint(host, 0, 32)
		if err != nil {
			return netip.Addr{}, false
		}
		return netip.AddrFrom4([4]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}), true
	}
	parts := strings.Split(host, ".")
	if len(parts) > 4 {
		return netip.Addr{}, false
	}
	nums := make([]uint64, 0, len(parts))
	allNumeric := true
	for _, p := range parts {
		if p == "" {
			return netip.Addr{}, false
		}
		// hex with 0x prefix, octal with leading 0 handled by base 0
		n, err := strconv.ParseUint(strings.ToLower(p), 0, 32)
		if err != nil {
			allNumeric = false
			break
		}
		nums = append(nums, n)
	}
	if !allNumeric {
		return netip.Addr{}, false
	}
	var n uint32
	switch len(nums) {
	case 2:
		if nums[0] > 255 || nums[1] > 1<<24-1 {
			return netip.Addr{}, false
		}
		n = uint32(nums[0])<<24 | uint32(nums[1])
	case 3:
		if nums[0] > 255 || nums[1] > 255 || nums[2] > 1<<16-1 {
			return netip.Addr{}, false
		}
		n = uint32(nums[0])<<24 | uint32(nums[1])<<16 | uint32(nums[2])
	case 4:
		for _, v := range nums {
			if v > 255 {
				return netip.Addr{}, false
			}
		}
		// if any part used non-canonical form, still treat as IP
		n = uint32(nums[0])<<24 | uint32(nums[1])<<16 | uint32(nums[2])<<8 | uint32(nums[3])
		return netip.AddrFrom4([4]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}), !isCanonicalDotted(host)
	default:
		return netip.Addr{}, false
	}
	return netip.AddrFrom4([4]byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}), true
}

func isCanonicalDotted(host string) bool {
	parts := strings.Split(host, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 || len(p) > 3 {
			return false
		}
		if len(p) > 1 && p[0] == '0' {
			return false
		}
		if strings.HasPrefix(strings.ToLower(p), "0x") {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
		v, _ := strconv.Atoi(p)
		if v > 255 {
			return false
		}
	}
	return true
}
