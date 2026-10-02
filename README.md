# GoToSocial sitemap.xml generator

GoToSocial sitemap generator written in Go. 100 lines of code. Deploy a single reproducible 6.8 MB binary.

## Setup

RSS won't work, you need the API, which requires authorization.

1. Create an OAuth app on your instance: `https://<your instance>/settings/user/applications/new`
   - In **Application name** put "Sitemap generator"
   - In **Redirect URIs** put `https://<your instance>/settings/user/applications/callback`
   - In **Scopes** put `read`
2. Click **Create**
3. Click on the created application in applications list
4. Scroll to **Request An API Access Token**
5. Ensure `read` is in **Token scopes**
6. Click **Request access token**
7. Authenticate with your GoToSocial account
8. Authorize application by clicking **Allow**
9. Click **I understand, show me the token!**
10. Copy it. Pass this token in the `TOKEN` environment variable (see below)

## Install

1. [Download go2social-sitemap](https://git.hloth.dev/hloth/go2social-sitemap/releases) to `/usr/local/bin/go2social-sitemap` or [build it yourself](./BUILDING.md)
2. Create a systemd unit file at `/etc/systemd/system/go2social-sitemap.service`:

Put your instance's URL to `GOTOSOCIAL_URL`, optionally edit `HOST` (defaults to `127.0.0.1`) and `PORT` (defaults to `3000`).

```ini
[Service]
ExecStart=/usr/local/bin/go2social-sitemap
Environment=GOTOSOCIAL_URL=https://m.hloth.dev
EnvironmentFile=/etc/go2social-sitemap/.env
#Environment=HOST=127.0.0.1
#Environment=PORT=3000
DynamicUser=yes
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

3. Create `/etc/go2social-sitemap/.env` (EnvironmentFile):

```env
# Paste the token from Setup step 10 in quotes
TOKEN=""
```

Make sure it's protected:

```sh
chmod 600 /etc/go2social-sitemap/.env
```

4. Enable the sitemap generator:

```sh
systemctl enable --now go2social-sitemap
```

5. Configure your reverse proxy to forward `https://<your instance>/sitemap.xml` requests to `http://127.0.0.1:3000` (HOST + PORT). E.g. Caddy with GoToSocial on `:8000` and the sitemap generator on `:3000`:

```caddyfile
m.hloth.dev {
	reverse_proxy /sitemap.xml :3000
	reverse_proxy :8000
}
```

Sitemaps are rebuilt every hour upon new request.

Not included in the sitemap: local-only posts, boosts, replies to other accounts, posts of accounts without "Mark account's posts as full-text indexable" checked in profile settings (off by default).

## License

[MIT](./LICENSE)
