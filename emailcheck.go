package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"
	"time"
)

// Strict email checks used ONLY when someone creates a new account.
// Login keeps the lenient ValidateEmail, so existing accounts with unusual
// addresses can still sign in.

// local part @ one or more DNS labels . TLD (letters only, 2+).
var emailRe = regexp.MustCompile(
	`^[A-Za-z0-9._%+\-]+@([A-Za-z0-9]([A-Za-z0-9\-]{0,61}[A-Za-z0-9])?\.)+[A-Za-z]{2,}$`)

const dnsCheckTimeout = 4 * time.Second

// Swapped out in tests.
var (
	lookupMX   = func(ctx context.Context, d string) ([]*net.MX, error) { return net.DefaultResolver.LookupMX(ctx, d) }
	lookupHost = func(ctx context.Context, d string) ([]string, error) { return net.DefaultResolver.LookupHost(ctx, d) }
)

// ValidateNewEmail checks the format and that the domain can receive mail.
func ValidateNewEmail(email string) error {
	if len(email) > 254 || !emailRe.MatchString(email) {
		return fmt.Errorf("Wrong email format")
	}
	domain := email[strings.LastIndex(email, "@")+1:]
	return checkMailDomain(domain)
}

func isDNSNotFound(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

// checkMailDomain looks for MX records, then falls back to A/AAAA (a domain
// without MX but with an address is still allowed to receive mail, RFC 5321).
// It only rejects when DNS clearly says the domain has nothing. If DNS itself
// is unreachable (offline, timeout) it lets the address through, so being
// offline never blocks sign-up.
func checkMailDomain(domain string) error {
	c, cancel := context.WithTimeout(context.Background(), dnsCheckTimeout)
	defer cancel()

	mx, err := lookupMX(c, domain)
	if len(mx) > 0 {
		// A single "." MX is the "this domain accepts no mail" marker (RFC 7505).
		if len(mx) == 1 && strings.TrimSuffix(mx[0].Host, ".") == "" {
			return fmt.Errorf("the email domain %q does not accept mail", domain)
		}
		return nil
	}
	if err != nil && !isDNSNotFound(err) {
		log.Printf("⚠️ MX lookup for %s failed (%v), skipping domain check", domain, err)
		return nil
	}

	hosts, err := lookupHost(c, domain)
	if len(hosts) > 0 {
		return nil
	}
	if err != nil && !isDNSNotFound(err) {
		log.Printf("⚠️ DNS lookup for %s failed (%v), skipping domain check", domain, err)
		return nil
	}

	return fmt.Errorf("the email domain %q cannot receive mail", domain)
}
