package libstandard

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
)

// Flag names registered by AddDatabaseFlags. Exported so applications can look
// them up on their own command trees if needed.
const (
	DBHost              = "db-host"
	DBPort              = "db-port"
	DBName              = "db-name"
	DBUser              = "db-user"
	DBPassword          = "db-password"
	DBSSLMode           = "db-sslmode"
	DBSSLRootCert       = "db-sslrootcert"
	DBSSLCert           = "db-sslcert"
	DBSSLKey            = "db-sslkey"
	DBMinConns          = "db-min-conns"
	DBMaxConns          = "db-max-conns"
	DBMaxConnLifetime   = "db-max-conn-lifetime"
	DBMaxConnIdleTime   = "db-max-conn-idle-time"
	DBHealthCheckPeriod = "db-health-check-period"
	DBConnectTimeout    = "db-connect-timeout"
)

// DatabaseConfig holds everything needed to build a pgx connection pool. The
// struct tags plug into this package's own config machinery (ReadFromEnv /
// ReadFromFile / ReadFromFlags), so every field carries a default that an
// application can override via config-file, environment variable or CLI flag
// without touching code.
//
// Durations are Go duration strings ("30s", "5m", "1h"). They intentionally
// default to the empty string, which means "keep pgx's own default" - see the
// warning on the pool fields below.
type DatabaseConfig struct {
	Host string `yaml:"host" env:"DB_HOST" env-default:"localhost" flag:"db-host"`
	Port uint16 `yaml:"port" env:"DB_PORT" env-default:"5432" flag:"db-port"`
	Name string `yaml:"name" env:"DB_NAME" env-default:"postgres" flag:"db-name"`
	User string `yaml:"user" env:"DB_USER" env-default:"postgres" flag:"db-user"`

	// Password drives the classic username/password path. Set it for password
	// auth; it is ignored once client-certificate auth is active (see below).
	Password string `yaml:"password" env:"DB_PASSWORD" flag:"db-password"`

	// TLS. Two mutually exclusive auth modes are supported:
	//
	//   - Password auth: leave SSLCert/SSLKey empty. The connection can still be
	//     encrypted via SSLMode (e.g. require or verify-full) plus SSLRootCert.
	//   - Client-certificate (mTLS) auth: set SSLCert and SSLKey. The files are
	//     reloaded on every TLS handshake, so short-lived rotating certs (e.g. a
	//     cert-manager CSI mount) work without a restart, and Password is unused.
	//
	// SSLMode follows libpq semantics (disable, allow, prefer, require,
	// verify-ca, verify-full).
	SSLMode     string `yaml:"sslMode" env:"DB_SSLMODE" env-default:"prefer" flag:"db-sslmode"`
	SSLRootCert string `yaml:"sslRootCert" env:"DB_SSLROOTCERT" flag:"db-sslrootcert"`
	SSLCert     string `yaml:"sslCert" env:"DB_SSLCERT" flag:"db-sslcert"`
	SSLKey      string `yaml:"sslKey" env:"DB_SSLKEY" flag:"db-sslkey"`

	// Pool sizing. MinConns keeps that many connections warm in the background,
	// so callers rarely pay a cold TCP+TLS handshake. MaxConns caps the pool;
	// when 0 pgx picks max(4, NumCPU).
	MinConns int32 `yaml:"minConns" env:"DB_MIN_CONNS" env-default:"2" flag:"db-min-conns"`
	MaxConns int32 `yaml:"maxConns" env:"DB_MAX_CONNS" env-default:"10" flag:"db-max-conns"`

	// WARNING: in pgxpool a duration of 0 means "expire immediately", NOT
	// "unlimited" (the opposite of database/sql). Leave these empty to inherit
	// pgx's sane defaults (lifetime 1h, idle 30m, health-check 1m) rather than
	// setting them to 0.
	MaxConnLifetime   string `yaml:"maxConnLifetime" env:"DB_MAX_CONN_LIFETIME" flag:"db-max-conn-lifetime"`
	MaxConnIdleTime   string `yaml:"maxConnIdleTime" env:"DB_MAX_CONN_IDLE_TIME" flag:"db-max-conn-idle-time"`
	HealthCheckPeriod string `yaml:"healthCheckPeriod" env:"DB_HEALTH_CHECK_PERIOD" flag:"db-health-check-period"`

	// ConnectTimeout bounds a single connection attempt. 0 means no timeout, so
	// it defaults to something finite.
	ConnectTimeout string `yaml:"connectTimeout" env:"DB_CONNECT_TIMEOUT" env-default:"10s" flag:"db-connect-timeout"`
}

// AddDatabaseFlags registers the database flags on a cobra command, mirroring
// AddVerbosityFlag / AddConfigFlag. The defaults here match the env-default
// tags on DatabaseConfig so that flags, env and file all agree.
func AddDatabaseFlags(cmd *cobra.Command) {
	f := cmd.PersistentFlags()
	f.String(DBHost, "localhost", "Database host.")
	f.Uint16(DBPort, 5432, "Database port.")
	f.String(DBName, "postgres", "Database name.")
	f.String(DBUser, "postgres", "Database user.")
	f.String(DBPassword, "", "Database password (omit when using client-certificate auth).")
	f.String(DBSSLMode, "prefer", "libpq sslmode: disable, allow, prefer, require, verify-ca, verify-full.")
	f.String(DBSSLRootCert, "", "Path to the CA certificate used to verify the server.")
	f.String(DBSSLCert, "", "Path to the client certificate (enables mTLS auth).")
	f.String(DBSSLKey, "", "Path to the client private key (enables mTLS auth).")
	f.Int32(DBMinConns, 2, "Minimum number of connections kept warm in the pool.")
	f.Int32(DBMaxConns, 10, "Maximum number of connections in the pool (0 = max(4, NumCPU)).")
	f.String(DBMaxConnLifetime, "", "Max connection lifetime, e.g. 1h (empty = pgx default; 0 means expire immediately).")
	f.String(DBMaxConnIdleTime, "", "Max idle time before a connection is closed, e.g. 30m (empty = pgx default).")
	f.String(DBHealthCheckPeriod, "", "Interval between pool health checks, e.g. 1m (empty = pgx default).")
	f.String(DBConnectTimeout, "10s", "Timeout for a single connection attempt (0 = none).")
}

// NewDatabasePool builds a *pgxpool.Pool from the config and verifies it with a
// ping. mTLS is wired up automatically when SSLCert and SSLKey are set, with
// hot-reloading of the client certificate on every handshake.
//
// For ent (or any database/sql consumer) wrap the returned pool:
//
//	pool, err := libstandard.NewDatabasePool(ctx, cfg)
//	...
//	db := libstandard.OpenDB(pool)
//	drv := entsql.OpenDB(dialect.Postgres, db)
//	client := ent.NewClient(ent.Driver(drv))
func NewDatabasePool(ctx context.Context, cfg DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := poolConfig(cfg)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return pool, nil
}

// OpenDB adapts a pool into a *sql.DB for database/sql consumers such as ent.
// The pool stays the single source of truth for connection management; closing
// the returned *sql.DB does not close the pool.
func OpenDB(pool *pgxpool.Pool) *sql.DB {
	return stdlib.OpenDBFromPool(pool)
}

// poolConfig translates a DatabaseConfig into a *pgxpool.Config. Split out from
// NewDatabasePool so it can be unit-tested without a live database.
func poolConfig(cfg DatabaseConfig) (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(buildConnString(cfg))
	if err != nil {
		return nil, err
	}

	poolCfg.MinConns = cfg.MinConns
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}

	for _, d := range []struct {
		name string
		val  string
		set  func(time.Duration)
	}{
		{"max-conn-lifetime", cfg.MaxConnLifetime, func(v time.Duration) { poolCfg.MaxConnLifetime = v }},
		{"max-conn-idle-time", cfg.MaxConnIdleTime, func(v time.Duration) { poolCfg.MaxConnIdleTime = v }},
		{"health-check-period", cfg.HealthCheckPeriod, func(v time.Duration) { poolCfg.HealthCheckPeriod = v }},
		{"connect-timeout", cfg.ConnectTimeout, func(v time.Duration) { poolCfg.ConnConfig.ConnectTimeout = v }},
	} {
		// Empty means "keep pgx's default"; only override on an explicit value.
		if d.val == "" {
			continue
		}
		parsed, err := time.ParseDuration(d.val)
		if err != nil {
			return nil, fmt.Errorf("invalid %s %q: %w", d.name, d.val, err)
		}
		d.set(parsed)
	}

	// pgx parses sslcert/sslkey once and pins the keypair into
	// TLSConfig.Certificates, which would never pick up a rotated cert. Replace
	// that with a hot-reloading GetClientCertificate while keeping everything
	// else pgx derived from sslmode (RootCAs, ServerName, verification). Every
	// TLS attempt - including the sslmode=prefer plaintext fallbacks - is
	// patched.
	if cfg.SSLCert != "" && cfg.SSLKey != "" {
		reloader := &clientCertReloader{certFile: cfg.SSLCert, keyFile: cfg.SSLKey}
		if _, err := reloader.load(); err != nil {
			return nil, fmt.Errorf("loading client certificate: %w", err)
		}

		patch := func(tlsCfg *tls.Config) {
			if tlsCfg == nil {
				return
			}
			tlsCfg.Certificates = nil
			tlsCfg.GetClientCertificate = reloader.getClientCertificate
		}
		patch(poolCfg.ConnConfig.TLSConfig)
		for _, fb := range poolCfg.ConnConfig.Fallbacks {
			patch(fb.TLSConfig)
		}
	}

	return poolCfg, nil
}

// buildConnString assembles a libpq-style URL from the config. Building it via
// net/url keeps user and password correctly escaped.
func buildConnString(cfg DatabaseConfig) string {
	u := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))),
		Path:   "/" + cfg.Name,
	}

	if cfg.User != "" {
		if cfg.Password != "" {
			u.User = url.UserPassword(cfg.User, cfg.Password)
		} else {
			u.User = url.User(cfg.User)
		}
	}

	q := u.Query()
	if cfg.SSLMode != "" {
		q.Set("sslmode", cfg.SSLMode)
	}
	if cfg.SSLRootCert != "" {
		q.Set("sslrootcert", cfg.SSLRootCert)
	}
	if cfg.SSLCert != "" {
		q.Set("sslcert", cfg.SSLCert)
	}
	if cfg.SSLKey != "" {
		q.Set("sslkey", cfg.SSLKey)
	}
	u.RawQuery = q.Encode()

	return u.String()
}

// clientCertReloader reads the client keypair from disk on every handshake so
// that a rotated certificate is picked up without a restart.
type clientCertReloader struct {
	certFile string
	keyFile  string

	mu     sync.RWMutex
	cached *tls.Certificate
}

// getClientCertificate is the tls.Config.GetClientCertificate callback.
func (r *clientCertReloader) getClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	cert, err := r.load()
	if err == nil {
		return cert, nil
	}

	// The CSI driver swaps tls.crt and tls.key atomically on renewal, but the
	// two reads here are not atomic together: a read landing across the swap
	// yields a mismatched pair. Serve the last good cert while it is still
	// valid instead of failing the connection over a microsecond-wide race.
	r.mu.RLock()
	cached := r.cached
	r.mu.RUnlock()
	if cached != nil && cached.Leaf != nil && time.Now().Before(cached.Leaf.NotAfter) {
		return cached, nil
	}

	return nil, err
}

// load reads and caches the keypair, populating Leaf so callers can check the
// expiry without re-parsing.
func (r *clientCertReloader) load() (*tls.Certificate, error) {
	/* #nosec G304 - cert paths are operator-supplied configuration by design */
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return nil, err
	}

	if cert.Leaf == nil && len(cert.Certificate) > 0 {
		if leaf, err := x509.ParseCertificate(cert.Certificate[0]); err == nil {
			cert.Leaf = leaf
		}
	}

	r.mu.Lock()
	r.cached = &cert
	r.mu.Unlock()

	return &cert, nil
}
