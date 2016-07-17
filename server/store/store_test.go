package store

import (
	"crypto/rand"
	"crypto/rsa"
	"os"
	"testing"
	"time"

	"github.com/nsheridan/cashier/testdata"
	"github.com/stretchr/testify/assert"

	"golang.org/x/crypto/ssh"
)

func TestParseCertificate(t *testing.T) {
	a := assert.New(t)
	now := uint64(time.Now().Unix())
	r, _ := rsa.GenerateKey(rand.Reader, 1024)
	pub, _ := ssh.NewPublicKey(r.Public())
	c := &ssh.Certificate{
		KeyId:           "id",
		ValidPrincipals: []string{"principal"},
		ValidBefore:     now,
		CertType:        ssh.UserCert,
		Key:             pub,
	}
	s, _ := ssh.NewSignerFromKey(r)
	c.SignCert(rand.Reader, s)
	rec := parseCertificate(c)

	a.Equal(c.KeyId, rec.KeyID)
	a.Equal(c.ValidPrincipals, rec.Principals)
	a.Equal(c.ValidBefore, rec.Expires)
	a.Equal(c.ValidAfter, rec.CreatedAt)
}

func testStore(t *testing.T, db CertStorer) {
	defer db.Close()

	ids := []string{"a", "b"}
	for _, id := range ids {
		r := &CertRecord{
			KeyID:   id,
			Expires: uint64(time.Now().UTC().Unix()) - 10,
		}
		if err := db.SetRecord(r); err != nil {
			t.Error(err)
		}
	}
	recs, err := db.List()
	if err != nil {
		t.Error(err)
	}
	if len(recs) != len(ids) {
		t.Errorf("Want %d records, got %d", len(ids), len(recs))
	}

	c, _, _, _, _ := ssh.ParseAuthorizedKey(testdata.Cert)
	cert := c.(*ssh.Certificate)
	cert.ValidBefore = uint64(time.Now().Add(1 * time.Hour).UTC().Unix())
	if err := db.SetCert(cert); err != nil {
		t.Error(err)
	}

	if _, err := db.Get("key"); err != nil {
		t.Error(err)
	}
	if err := db.Revoke("key"); err != nil {
		t.Error(err)
	}

	// A revoked key shouldn't get returned if it's already expired
	db.Revoke("a")

	revoked, err := db.GetRevoked()
	if err != nil {
		t.Error(err)
	}
	for _, k := range revoked {
		if k.KeyID != "key" {
			t.Errorf("Unexpected key: %s", k.KeyID)
		}
	}
}

func TestMemoryStore(t *testing.T) {
	db := NewMemoryStore()
	testStore(t, db)
}

func TestMySQLStore(t *testing.T) {
	config := os.Getenv("MYSQL_TEST_CONFIG")
	if config == "" {
		t.Skip("No MYSQL_TEST_CONFIG environment variable")
	}
	db, err := NewMySQLStore(config)
	if err != nil {
		t.Error(err)
	}
	testStore(t, db)
}