package main

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
)

func TestTheAuthorityIsKeptAndTheCertificateFollowsTheHosts(t *testing.T) {
	dir := t.TempDir()
	first, err := certificate(dir, []string{"localhost", "192.168.1.5"})
	if err != nil {
		t.Fatal(err)
	}
	authority, _ := os.ReadFile(filepath.Join(dir, authorityFile))

	same, err := certificate(dir, []string{"localhost", "192.168.1.5"})
	if err != nil || string(same.Certificate[0]) != string(first.Certificate[0]) {
		t.Fatalf("the same hosts should keep the certificate, err = %v", err)
	}

	moved, err := certificate(dir, []string{"localhost", "10.0.0.7"})
	if err != nil || string(moved.Certificate[0]) == string(first.Certificate[0]) {
		t.Fatalf("a new address should make a new certificate, err = %v", err)
	}
	if kept, _ := os.ReadFile(filepath.Join(dir, authorityFile)); string(kept) != string(authority) {
		t.Fatal("the authority a phone trusts must stay the same")
	}

	leaf, err := x509.ParseCertificate(moved.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	ca, _, err := loadAuthority(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := leaf.CheckSignatureFrom(ca); err != nil {
		t.Fatalf("the certificate should be signed by the authority: %v", err)
	}
	if len(leaf.IPAddresses) != 1 || leaf.IPAddresses[0].String() != "10.0.0.7" || len(leaf.DNSNames) != 1 {
		t.Fatalf("the certificate should name the hosts, got %v %v", leaf.DNSNames, leaf.IPAddresses)
	}
}
