package libstandard

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildConnString(t *testing.T) {
	cfg := DatabaseConfig{
		Host: "db.example", Port: 5432, Name: "apps", User: "cryptdoc",
		SSLMode: "verify-full", SSLRootCert: "/run/ca.crt",
		SSLCert: "/run/tls.crt", SSLKey: "/run/tls.key",
	}

	got := buildConnString(cfg)
	assert.Contains(t, got, "postgres://cryptdoc@db.example:5432/apps")
	assert.Contains(t, got, "sslmode=verify-full")
	assert.Contains(t, got, "sslcert=%2Frun%2Ftls.crt")
	assert.Contains(t, got, "sslkey=%2Frun%2Ftls.key")
	assert.Contains(t, got, "sslrootcert=%2Frun%2Fca.crt")
	assert.NotContains(t, got, "password")
}

func TestBuildConnStringEscapesPassword(t *testing.T) {
	cfg := DatabaseConfig{Host: "h", Port: 5432, Name: "d", User: "u", Password: "p@ss:w/rd"}
	got := buildConnString(cfg)
	assert.Contains(t, got, "u:p%40ss%3Aw%2Frd@h:5432")
}

func TestDatabaseConfigDefaultsFromEnv(t *testing.T) {
	var cfg DatabaseConfig
	require.NoError(t, ReadFromEnv(&cfg))

	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, uint16(5432), cfg.Port)
	assert.Equal(t, "prefer", cfg.SSLMode)
	assert.Equal(t, int32(2), cfg.MinConns)
	assert.Equal(t, int32(10), cfg.MaxConns)
	assert.Equal(t, "10s", cfg.ConnectTimeout)
	// Durations default to empty so pgx keeps its own sane defaults.
	assert.Empty(t, cfg.MaxConnLifetime)
}

func TestDatabaseConfigEnvOverride(t *testing.T) {
	t.Setenv("DB_HOST", "db.internal")
	t.Setenv("DB_MIN_CONNS", "5")
	t.Setenv("DB_MAX_CONN_LIFETIME", "2h")

	var cfg DatabaseConfig
	require.NoError(t, ReadFromEnv(&cfg))

	assert.Equal(t, "db.internal", cfg.Host)
	assert.Equal(t, int32(5), cfg.MinConns)
	assert.Equal(t, "2h", cfg.MaxConnLifetime)
}

func TestPoolConfigAppliesSettings(t *testing.T) {
	cfg := DatabaseConfig{
		Host: "localhost", Port: 5432, Name: "apps", User: "u", SSLMode: "disable",
		MinConns: 3, MaxConns: 12,
		MaxConnLifetime: "2h", MaxConnIdleTime: "10m", HealthCheckPeriod: "45s",
		ConnectTimeout: "7s",
	}

	pc, err := poolConfig(cfg)
	require.NoError(t, err)

	assert.Equal(t, int32(3), pc.MinConns)
	assert.Equal(t, int32(12), pc.MaxConns)
	assert.Equal(t, 2*time.Hour, pc.MaxConnLifetime)
	assert.Equal(t, 10*time.Minute, pc.MaxConnIdleTime)
	assert.Equal(t, 45*time.Second, pc.HealthCheckPeriod)
	assert.Equal(t, 7*time.Second, pc.ConnConfig.ConnectTimeout)
}

func TestPoolConfigKeepsPgxDefaultsWhenDurationsEmpty(t *testing.T) {
	cfg := DatabaseConfig{Host: "localhost", Port: 5432, Name: "apps", User: "u", SSLMode: "disable"}

	pc, err := poolConfig(cfg)
	require.NoError(t, err)

	// pgx's own defaults - crucially NOT zero, which would mean "expire now".
	assert.Equal(t, time.Hour, pc.MaxConnLifetime)
	assert.Equal(t, 30*time.Minute, pc.MaxConnIdleTime)
}

func TestPoolConfigInvalidDuration(t *testing.T) {
	cfg := DatabaseConfig{Host: "localhost", Port: 5432, Name: "d", User: "u", MaxConnLifetime: "nonsense"}
	_, err := poolConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max-conn-lifetime")
}

func TestPoolConfigPasswordAuthNoClientCert(t *testing.T) {
	cfg := DatabaseConfig{
		Host: "localhost", Port: 5432, Name: "apps", User: "cryptdoc",
		Password: "s3cret", SSLMode: "require",
	}

	pc, err := poolConfig(cfg)
	require.NoError(t, err)

	assert.Equal(t, "cryptdoc", pc.ConnConfig.User)
	assert.Equal(t, "s3cret", pc.ConnConfig.Password)
	// Encryption is on (sslmode=require), but no client cert is presented.
	require.NotNil(t, pc.ConnConfig.TLSConfig)
	assert.Nil(t, pc.ConnConfig.TLSConfig.GetClientCertificate)
}

func TestPoolConfigPasswordAuthSSLDisabled(t *testing.T) {
	cfg := DatabaseConfig{
		Host: "localhost", Port: 5432, Name: "apps", User: "u",
		Password: "pw", SSLMode: "disable",
	}

	pc, err := poolConfig(cfg)
	require.NoError(t, err)

	assert.Equal(t, "pw", pc.ConnConfig.Password)
	assert.Nil(t, pc.ConnConfig.TLSConfig)
}

func TestPoolConfigWiresClientCertReloader(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeKeyPair(t, dir, "tls")

	cfg := DatabaseConfig{
		Host: "localhost", Port: 5432, Name: "apps", User: "u",
		SSLMode: "require", SSLCert: certFile, SSLKey: keyFile,
	}

	pc, err := poolConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, pc.ConnConfig.TLSConfig)
	assert.Nil(t, pc.ConnConfig.TLSConfig.Certificates)
	assert.NotNil(t, pc.ConnConfig.TLSConfig.GetClientCertificate)

	cert, err := pc.ConnConfig.TLSConfig.GetClientCertificate(nil)
	require.NoError(t, err)
	assert.NotNil(t, cert.Leaf)
}

func TestPoolConfigMissingClientCertFails(t *testing.T) {
	cfg := DatabaseConfig{
		Host: "localhost", Port: 5432, Name: "apps", User: "u",
		SSLMode: "require", SSLCert: "/nonexistent/tls.crt", SSLKey: "/nonexistent/tls.key",
	}
	_, err := poolConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tls.key")
}

func TestClientCertReloaderServesCachedAcrossSwap(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeKeyPair(t, dir, "tls")

	r := &clientCertReloader{certFile: certFile, keyFile: keyFile}
	_, err := r.load()
	require.NoError(t, err)

	// Simulate the swap window: key on disk no longer matches the cert.
	_, otherKey := writeKeyPair(t, dir, "other")
	require.NoError(t, os.Rename(otherKey, keyFile))

	// Direct load now fails (mismatched pair), but the callback falls back to
	// the still-valid cached cert rather than erroring.
	_, loadErr := r.load()
	require.Error(t, loadErr)

	cert, err := r.getClientCertificate(nil)
	require.NoError(t, err)
	assert.NotNil(t, cert)
}

func TestAddDatabaseFlags(t *testing.T) {
	cmd := &cobra.Command{}
	AddDatabaseFlags(cmd)

	for _, name := range []string{DBHost, DBPort, DBMinConns, DBMaxConns, DBSSLCert, DBConnectTimeout} {
		assert.NotNil(t, cmd.PersistentFlags().Lookup(name), "flag %q should be registered", name)
	}
}

// writeKeyPair writes a self-signed cert/key pair into dir and returns their
// paths. Used to exercise the TLS wiring without a real CA.
func writeKeyPair(t *testing.T, dir, name string) (certFile, keyFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	certFile = filepath.Join(dir, name+".crt")
	keyFile = filepath.Join(dir, name+".key")

	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))

	return certFile, keyFile
}
