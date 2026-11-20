// Package clientip resolves the client IP of a request from the proxies set
// in REEARTH_ACCOUNTS_TRUSTED_PROXIES.
package clientip

import (
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/labstack/echo/v4"
)

// Extractor reads the client IP from the right-most X-Forwarded-For hop that
// is not a listed proxy. Echo trusts loopback, link-local and private ranges
// by default; those defaults are turned off so that only the listed proxies
// are trusted.
func Extractor(proxies []netip.Prefix) echo.IPExtractor {
	if len(proxies) == 0 {
		return echo.ExtractIPDirect()
	}
	opts := []echo.TrustOption{
		echo.TrustLoopback(false),
		echo.TrustLinkLocal(false),
		echo.TrustPrivateNet(false),
	}
	for _, p := range proxies {
		opts = append(opts, echo.TrustIPRange(&net.IPNet{
			IP:   p.Addr().AsSlice(),
			Mask: net.CIDRMask(p.Bits(), p.Addr().BitLen()),
		}))
	}
	return echo.ExtractIPFromXFFHeader(opts...)
}

// ParseTrustedProxies parses CIDRs and single IPs. Empty entries are skipped.
// A prefix that covers every IPv4 or IPv6 address is rejected, because it
// would let every client choose its IP. ::ffff:0:0/96 counts: Echo matches
// IPv4 peers against IPv4-mapped prefixes.
func ParseTrustedProxies(entries []string) ([]netip.Prefix, error) {
	var res []netip.Prefix
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if strings.Contains(e, "/") {
			p, err := netip.ParsePrefix(e)
			if err != nil {
				return nil, err
			}
			p = p.Masked()
			if p.Bits() == 0 || (p.Addr().Is4In6() && p.Bits() <= 96) {
				return nil, fmt.Errorf("%q covers every IPv4 or IPv6 address", e)
			}
			res = append(res, p)
			continue
		}
		a, err := netip.ParseAddr(e)
		if err != nil {
			return nil, err
		}
		if a.Zone() != "" {
			return nil, fmt.Errorf("address %q must not have a zone", e)
		}
		res = append(res, netip.PrefixFrom(a, a.BitLen()))
	}
	return res, nil
}
