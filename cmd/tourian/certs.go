package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Phones trust the platform the way they trust any local authority: a certificate authority of the platform's own, made once and kept, signs a server certificate for the hosts the platform is reached by, made again whenever those change. The authority is what a phone installs; Apple takes server certificates of at most 825 days.
const (
	authorityLifetime   = 10 * 365 * 24 * time.Hour
	certificateLifetime = 800 * 24 * time.Hour
	authorityFile       = "ca.pem"
	authorityKeyFile    = "ca-key.pem"
	certificateFile     = "cert.pem"
	keyFile             = "key.pem"
)

// A keeper holds the server certificate and remakes it when the hosts the platform is reached by change while it runs, so a laptop that moves to another network keeps a certificate that fits.
type keeper struct {
	dir   string
	hosts func() []string
	mu    sync.RWMutex
	cert  *tls.Certificate
	names []string
}

func (k *keeper) refresh() error {
	hosts := k.hosts()
	k.mu.RLock()
	same := slices.Equal(hosts, k.names)
	k.mu.RUnlock()
	if same {
		return nil
	}
	cert, err := certificate(k.dir, hosts)
	if err != nil {
		return err
	}
	k.mu.Lock()
	k.cert, k.names = &cert, hosts
	k.mu.Unlock()
	return nil
}

func (k *keeper) get(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.cert, nil
}

// follow refreshes every so often until ctx ends; a refresh that fails keeps the certificate in hand.
func (k *keeper) follow(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := k.refresh(); err != nil {
				log.Printf("tls: %v", err)
			}
		}
	}
}

// certificate returns a server certificate for hosts, signed by the authority under dir, making either as needed.
func certificate(dir string, hosts []string) (tls.Certificate, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, err
	}
	authority, key, err := loadAuthority(dir)
	if errors.Is(err, os.ErrNotExist) {
		authority, key, err = makeAuthority(dir)
	}
	if err != nil {
		return tls.Certificate{}, err
	}
	if cert, err := tls.LoadX509KeyPair(filepath.Join(dir, certificateFile), filepath.Join(dir, keyFile)); err == nil {
		if leaf, err := x509.ParseCertificate(cert.Certificate[0]); err == nil && covers(leaf, hosts) && leaf.NotAfter.After(time.Now().Add(24*time.Hour)) && leaf.CheckSignatureFrom(authority) == nil {
			return cert, nil
		}
	}
	return makeCertificate(dir, authority, key, hosts)
}

func covers(leaf *x509.Certificate, hosts []string) bool {
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			if !slices.ContainsFunc(leaf.IPAddresses, ip.Equal) {
				return false
			}
		} else if !slices.Contains(leaf.DNSNames, host) {
			return false
		}
	}
	return true
}

func loadAuthority(dir string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	pair, err := tls.LoadX509KeyPair(filepath.Join(dir, authorityFile), filepath.Join(dir, authorityKeyFile))
	if err != nil {
		return nil, nil, err
	}
	authority, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, nil, err
	}
	key, ok := pair.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, nil, errors.New("tls: the authority's key is not ECDSA")
	}
	return authority, key, nil
}

func makeAuthority(dir string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "tourian local authority", Organization: []string{"tourian"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(authorityLifetime),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	if err := writePair(dir, authorityFile, authorityKeyFile, der, key); err != nil {
		return nil, nil, err
	}
	authority, err := x509.ParseCertificate(der)
	return authority, key, err
}

func makeCertificate(dir string, authority *x509.Certificate, authorityKey *ecdsa.PrivateKey, hosts []string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: "tourian", Organization: []string{"tourian"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(certificateLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, authority, &key.PublicKey, authorityKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := writePair(dir, certificateFile, keyFile, der, key); err != nil {
		return tls.Certificate{}, err
	}
	return tls.LoadX509KeyPair(filepath.Join(dir, certificateFile), filepath.Join(dir, keyFile))
}

func writePair(dir, certName, keyName string, der []byte, key *ecdsa.PrivateKey) error {
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(filepath.Join(dir, certName), certPEM, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, keyName), keyPEM, 0o600)
}

func serial() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	return n
}

// lanHosts is every name the platform can be reached by from this machine and the local network: localhost, the hostname, and each address of an interface that is up.
func lanHosts() []string {
	hosts := []string{"localhost", "127.0.0.1", "::1"}
	if name, err := os.Hostname(); err == nil && name != "" {
		hosts = append(hosts, name)
	}
	for _, ip := range lanAddresses() {
		hosts = append(hosts, ip.String())
	}
	return hosts
}

// lanAddresses are the machine's addresses on the local network, IPv4 first.
func lanAddresses() []net.IP {
	var v4, v6 []net.IP
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ip, ok := addr.(*net.IPNet)
			if !ok || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() {
				continue
			}
			if ip.IP.To4() != nil {
				v4 = append(v4, ip.IP)
			} else {
				v6 = append(v6, ip.IP)
			}
		}
	}
	return append(v4, v6...)
}
