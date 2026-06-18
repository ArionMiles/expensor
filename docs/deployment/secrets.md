# Secrets

Expensor encrypts reader client secrets and OAuth tokens before storing them. Encryption key material is mandatory once encrypted reader runtime storage is enabled.

Generate a key:

```bash
task secrets:generate
```

This prints a base64-encoded 32-byte key. Store it outside the database and back it up. If the key is lost, stored reader credentials cannot be decrypted and readers must be reconnected.

For simple `.env` deployments:

```env
EXPENSOR_SECRET_KEY=base64-encoded-key-here
```

For Docker Compose secret-file deployments, mount a file containing only the base64-encoded key and point Expensor at it:

```env
EXPENSOR_SECRET_KEY_FILE=/run/secrets/expensor_secret_key
```

Only set one of `EXPENSOR_SECRET_KEY` or `EXPENSOR_SECRET_KEY_FILE`. Startup fails when both are set or when the decoded key is not exactly 32 bytes.

Docker secrets and bind-mounted secret files still exist on the host disk. This is an acceptable self-hosted tradeoff for Expensor: it reduces accidental exposure through environment dumps and protects against database-only compromise, but it does not protect against full host compromise.

External secret managers can work with Expensor by exporting `EXPENSOR_SECRET_KEY` before startup or by rendering a file consumed through `EXPENSOR_SECRET_KEY_FILE`.
