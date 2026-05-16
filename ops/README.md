# ConfigHub Ops Quickstart

## Local Docker build

```sh
docker build -t confighub:dev .
docker run --rm confighub:dev --version
```

## Ship to `hk-cn2`

```sh
docker save confighub:dev | gzip | ssh hk-cn2 'gunzip | docker load'
```

## Prepare state and token

Create a token before exposing a non-loopback bind:

```sh
confighub token create --label macbook --scope pull:macbook --root /var/lib/confighub
```

The plaintext token is printed once. Store it in your password manager.

## Run on `hk-cn2` with Docker

```sh
docker run -d --name confighub \
  -p 127.0.0.1:8787:8787 \
  -v /var/lib/confighub/profiles:/var/lib/confighub/profiles \
  -v /var/lib/confighub/templates:/var/lib/confighub/templates \
  -v /var/lib/confighub/bundles:/var/lib/confighub/bundles \
  -v /var/lib/confighub/state:/var/lib/confighub/state \
  confighub:dev serve --bind 0.0.0.0:8787 --root /var/lib/confighub
```

If no token exists, this command fails with exit code 21 by design.

## Reverse proxy with Caddy

Use `ops/caddy/Caddyfile.example` as a minimal TLS termination example:

```sh
caddy run --config ops/caddy/Caddyfile.example
```

The ConfigHub binary logs a warning for non-loopback plain HTTP binds; serve real traffic through Caddy or another TLS reverse proxy.

## Systemd-only deployment

1. Install the binary at `/usr/local/bin/confighub`.
2. Create user and directories:
   ```sh
   sudo useradd --system --home /var/lib/confighub --shell /usr/sbin/nologin confighub
   sudo install -d -o confighub -g confighub -m 0700 /var/lib/confighub/{profiles,templates,bundles,state}
   ```
3. Copy `ops/systemd/confighub.service` to `/etc/systemd/system/confighub.service`.
4. Create tokens with `confighub token create --root /var/lib/confighub`.
5. Start the service:
   ```sh
   sudo systemctl daemon-reload
   sudo systemctl enable --now confighub
   ```
