package signer

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server/store"
	"github.com/nsheridan/cashier/testdata"
	"github.com/stripe/krl"

	"golang.org/x/crypto/ssh"
)

var (
	key, _ = ssh.ParsePrivateKey(testdata.Priv)
	signer = &KeySigner{
		ca:         key,
		validity:   12 * time.Hour,
		principals: []string{"ec2-user"},
	}
)

func TestCert(t *testing.T) {
	t.Parallel()
	r := &lib.SignRequest{
		Key:        string(testdata.Pub),
		Principal:  "gopher1",
		ValidUntil: time.Now().Add(1 * time.Hour),
	}
	cert, err := signer.SignUserKey(r)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(cert.SignatureKey.Marshal(), signer.ca.PublicKey().Marshal()) {
		t.Error("Cert signer and server signer don't match")
	}
	var principals []string
	principals = append(principals, r.Principal)
	principals = append(principals, signer.principals...)
	if !reflect.DeepEqual(cert.ValidPrincipals, principals) {
		t.Errorf("Expected %s, got %s", cert.ValidPrincipals, principals)
	}
	k1, _, _, _, err := ssh.ParseAuthorizedKey([]byte(r.Key))
	k2 := cert.Key
	if !bytes.Equal(k1.Marshal(), k2.Marshal()) {
		t.Error("Cert key doesn't match public key")
	}
	if cert.ValidBefore != uint64(r.ValidUntil.Unix()) {
		t.Errorf("Invalid validity, expected %d, got %d", r.ValidUntil, cert.ValidBefore)
	}
}

func TestRevocationList(t *testing.T) {
	t.Parallel()
	r := &lib.SignRequest{
		Key:        string(testdata.Pub),
		Principal:  "revoked",
		ValidUntil: time.Now().Add(1 * time.Hour),
	}
	cert1, _ := signer.SignUserKey(r)
	r.Principal = "ok"
	cert2, _ := signer.SignUserKey(r)
	var rec []*store.CertRecord
	rec = append(rec, &store.CertRecord{
		KeyID: cert1.KeyId,
	})
	rl, err := signer.GenerateRevocationList(rec)
	if err != nil {
		t.Error(err)
	}
	k, err := krl.ParseKRL(rl)
	if err != nil {
		t.Error(err)
	}
	if !k.IsRevoked(cert1) {
		t.Errorf("expected cert %s to be revoked", cert1.KeyId)
	}
	if k.IsRevoked(cert2) {
		t.Errorf("cert %s should not be revoked", cert2.KeyId)
	}
}
