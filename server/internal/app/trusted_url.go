package app

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"sort"
	"strings"

	"github.com/reearth/reearthx/appx"
)

// checkTrustedURL passes only an https URL on a DNS name (ADR-0002 decision
// 11.6). Errors may name the host but never the userinfo, path, or query.
func checkTrustedURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		// url.Parse errors quote the whole input, which may carry credentials.
		return errors.New("not a valid URL")
	}
	if u.Scheme != "https" {
		return errors.New("scheme must be https")
	}
	if u.User != nil {
		return errors.New("must not contain userinfo")
	}
	return checkTrustedHost(u.Hostname())
}

func checkTrustedHost(host string) error {
	if host == "" {
		return errors.New("host is empty")
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return fmt.Errorf("host %q is an IP address", host)
	}
	labels := strings.Split(strings.TrimSuffix(strings.ToLower(host), "."), ".")
	for _, l := range labels {
		if l == "" {
			return fmt.Errorf("host %q has an empty label", host)
		}
		// ASCII only: an HTTP client maps other dots (such as U+3002) to "."
		// before it resolves the name, so they would hide a label from the
		// checks below.
		if strings.ContainsFunc(l, func(r rune) bool { return !isLetter(r) && !isDigit(r) && r != '-' }) {
			return fmt.Errorf("host %q is not an ASCII DNS name", host)
		}
		// localhost.localdomain and localhost.<search domain> commonly
		// resolve to loopback too.
		if l == "localhost" {
			return fmt.Errorf("host %q is localhost", host)
		}
	}
	if len(labels) < 2 {
		return fmt.Errorf("host %q is not a DNS name with a dot", host)
	}
	// A last label that starts with a digit lets a C resolver read the name
	// as an IPv4 address in a short or hex form such as 127.1 or 127.0.0.0x1.
	if !isLetter(rune(labels[len(labels)-1][0])) {
		return fmt.Errorf("host %q does not end in a top-level domain", host)
	}
	return nil
}

func isLetter(r rune) bool { return 'a' <= r && r <= 'z' }

func isDigit(r rune) bool { return '0' <= r && r <= '9' }

func checkTrustedURLs(providers []appx.JWTProvider, callers ServiceCallers) error {
	var errs []error
	check := func(source, raw string) {
		if err := checkTrustedURL(raw); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", source, err))
		}
	}
	for i, p := range providers {
		check(fmt.Sprintf("auth provider %d issuer", i+1), p.ISS)
		if p.JWKSURI != nil && *p.JWKSURI != "" {
			check(fmt.Sprintf("auth provider %d jwks uri", i+1), *p.JWKSURI)
		}
	}
	keys := make([]serviceCallerKey, 0, len(callers))
	for k := range callers {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].clientID != keys[j].clientID {
			return keys[i].clientID < keys[j].clientID
		}
		return keys[i].issuer < keys[j].issuer
	})
	for _, k := range keys {
		check(fmt.Sprintf("service caller %q issuer", k.clientID), k.issuer)
	}
	return errors.Join(errs...)
}
