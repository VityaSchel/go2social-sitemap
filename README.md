# Gotosocial sitemap.xml generator

## Usage

RSS won't work, you need API, which requires authorization.

Replace `<yourinstance>` with your gotosocial instance's frontend.

1. Create an OAuth app here: `https://<yourinstance>/settings/user/applications/new`
   - Put "Sitemap generator" as "Application name"
   - Put `https://<yourinstance>/settings/user/applications/callback` to "Redirect URIs"
   - Put `read` to "Scopes"
2. Click on the created application in applications list
3. Scroll to "Request An API Access Token"
4. Ensure `read` is in "Token scopes"
5. Click "Request access token"
6. Authenticate with your gotosocial account
7. Authorize application by clicking "Allow"
8. Click "I understand, show me the token!"
9. Copy it
10. Run `install -m 600 .env.example .env` in repository's directory
11. Open .env and paste the copied access token into quotes of `TOKEN=""` variable
12. Run `PORT=1234 bun index.ts` in repository's directory
13. Now try making a GET http://localhost:1234/sitemap.xml request

Configure your reverse proxy to redirect /sitemap.xml request to localhost:1234.

Sitemaps are rebuilt every hour upon new request.

## License

[MIT](./LICENSE)

## Donate

[hloth.dev/donate](https://hloth.dev/donate)