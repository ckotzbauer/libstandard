
# libstandard

> Shared code for config-handling (lightweight viper-replacement), a pgx-based database pool and utility functions.

[![test](https://github.com/ckotzbauer/libstandard/actions/workflows/test.yml/badge.svg)](https://github.com/ckotzbauer/libstandard/actions/workflows/test.yml)


## Database (pgx v5)

`NewDatabasePool` builds a `*pgxpool.Pool` from a `DatabaseConfig`. Two auth
modes are supported and picked automatically:

- **Username/password** — the classic path. Set `User`/`Password`, leave
  `SSLCert`/`SSLKey` empty. The link can still be encrypted via `SSLMode`
  (`require`, `verify-full`, …) and `SSLRootCert`.
- **Client certificate (mTLS)** — set `SSLCert`/`SSLKey`. When they point at a
  rotating cert (e.g. a cert-manager CSI mount) the keypair is reloaded on every
  TLS handshake, so certs rotate without a restart, and `Password` is unused.

`MinConns` keeps connections warm in the background to avoid cold-handshake
latency.

`DatabaseConfig` uses the same tags as the rest of the config machinery, so
every value has a default that env, file or CLI flag can override. Secrets can
come from files instead of the environment: every `env` var also honors a
`<VAR>_FILE` fallback, so set `DB_PASSWORD_FILE=/var/run/secrets/db/password`
(one trailing newline stripped, 1 MiB cap) and volume-mounted k8s secrets never
appear as ENV. **Caution:** a field tagged `env:"VAR"` treats an existing
`VAR_FILE` variable as a secret path — do not reuse `VAR_FILE` names for other
purposes in the same app.

```go
var cfg struct {
    libstandard.DatabaseConfig
}
if err := libstandard.ReadFromEnv(&cfg); err != nil { /* ... */ }

pool, err := libstandard.NewDatabasePool(ctx, cfg.DatabaseConfig)
if err != nil { /* ... */ }
defer pool.Close()

// For ent / database/sql:
db := libstandard.OpenDB(pool)
drv := entsql.OpenDB(dialect.Postgres, db)
client := ent.NewClient(ent.Driver(drv))
```

To expose the settings as CLI flags, call `libstandard.AddDatabaseFlags(cmd)`.

> **Note:** in pgxpool a connection lifetime/idle-time of `0` means *expire
> immediately*, not *unlimited*. Leave `MaxConnLifetime`/`MaxConnIdleTime` empty
> to inherit pgx's sane defaults (1h / 30m).


## Single values

Apps that do not bind config through a struct (viper, plain `os.Getenv`) can
still use the `<VAR>_FILE` convention through `EnvOrFile`:

```go
apiKey, err := libstandard.EnvOrFile("API_KEY")
if err != nil { /* broken secret mount */ }
```

The value comes from the `API_KEY` env var when set, otherwise from the file
`API_KEY_FILE` points at. Unset both yield `""`; a missing or oversized file
returns an error instead of silently producing an empty secret.


## Security

When discovering security issues please refer to the [Security process](https://github.com/ckotzbauer/.github/blob/main/SECURITY.md).


[License](https://github.com/ckotzbauer/libstandard/blob/main/LICENSE)
--------


## Contributing

Please refer to the [Contribution guildelines](https://github.com/ckotzbauer/.github/blob/main/CONTRIBUTING.md).

## Code of conduct

Please refer to the [Conduct guildelines](https://github.com/ckotzbauer/.github/blob/main/CODE_OF_CONDUCT.md).


## Credits

The config-code is mostly ported from the great [cleanenv](https://github.com/ilyakaznacheev/cleanenv) library which is [MIT licensed](https://github.com/ilyakaznacheev/cleanenv/blob/master/LICENSE).
