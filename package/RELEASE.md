# aux4/curl 1.3.1

## Added

- **`oauth login --broker <url>`** — an OAuth-broker-backed login. The broker
  holds the client secret: it builds the authorization URL (returning the PKCE
  verifier) and performs the code exchange, while curl runs the same local
  loopback callback as the normal flow. The stored token records a `broker` grant
  plus the broker URL, so **this client never handles a client secret** — at login
  or at refresh. `--clientId` / `--authUrl` / `--tokenUrl` are not needed in this
  mode.

- **`grant: broker` token refresh** — when a broker-obtained access token expires,
  curl renews it by POSTing the refresh token back to the broker's
  `POST {brokerUrl}/{provider}/refresh` route (the broker holds the secret),
  rotating the refresh token when the provider returns a new one.

## Internal

- Extracted the loopback callback listener into a shared helper reused by both the
  local and broker login flows (no behaviour change to the existing flow).
