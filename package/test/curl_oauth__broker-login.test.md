# curl oauth login --broker

The `--broker` mode delegates the client secret to an OAuth broker: the broker
builds the authorization URL (returning the PKCE verifier), then the browser lands
on the broker's OWN callback which parks the code, and this client polls the broker
for it before exchanging it for tokens — so it works from any device with no
loopback, and this client never handles a secret. These tests stand up a tiny local
Node mock broker (`mock-broker-server.js`) that mimics `aux4/oauth-broker`'s routes
on `localhost:8799`; its `/session/:id` route returns the parked code immediately,
standing in for the browser having completed consent.

```beforeAll
nohup node mock-broker-server.js >/dev/null 2>&1 &
REQUIRE_AUTH=1 AUTH_FILE=authseen.txt MOCK_BROKER_PORT=8798 nohup node mock-broker-server.js >/dev/null 2>&1 &
sleep 1
mkdir -p .oauth
```

```afterAll
pkill -f mock-broker-server.js
rm -rf .oauth login.log authseen.txt
```

## broker login (poll)

### should obtain and store a broker-grant token via the poll flow

The mock on `8799` is a public broker, so no `--brokerToken` is passed and the
client sends no Authorization header (a secured broker is covered separately below).

```execute
rm -f .oauth/google.json
aux4 curl oauth login google --scopes "openid email" --broker http://localhost:8799 --tokenFile .oauth/google.json >login.log 2>&1
cat .oauth/google.json
```

```expect:partial
"accessToken": "at-1"
```

### should record the broker grant and URL on the token

```execute
cat .oauth/google.json
```

```expect:partial
"grant": "broker"
```

## secured broker

The mock on `8798` runs with `REQUIRE_AUTH`, so every route returns 401 unless the
client sends `Authorization: Bearer <token>`. The caller supplies the token via
`--brokerToken`; curl itself never decides where the token comes from.

### should send the bearer token and log in against a secured broker

```execute
rm -f .oauth/g2.json
aux4 curl oauth login google --scopes "openid email" --broker http://localhost:8798 --brokerToken tok-abc --tokenFile .oauth/g2.json >login.log 2>&1
cat .oauth/g2.json
```

```expect:partial
"accessToken": "at-1"
```

### should have sent the token as a Bearer credential

```execute
cat authseen.txt
```

```expect
Bearer tok-abc
```

### should fail with a clear message when no broker token is supplied

```execute
aux4 curl oauth login google --scopes "openid email" --broker http://localhost:8798 --tokenFile .oauth/g3.json
```

```error:partial
requires authentication
```

## refresh via broker

### should renew an expired broker-grant token through the broker

```beforeAll
mkdir -p .oauth
cat > .oauth/brokerrefresh.json << 'ENDTOKEN'
{
  "grant": "broker",
  "brokerUrl": "http://localhost:8799",
  "provider": "google",
  "scopes": "openid email",
  "accessToken": "at-old",
  "refreshToken": "rt-old",
  "expiresAt": "2020-01-01T00:00:00Z"
}
ENDTOKEN
```

```execute
aux4 curl oauth token brokerrefresh
```

```expect
at-new
```

### should surface a broker refresh error

```beforeAll
mkdir -p .oauth
cat > .oauth/brokerbad.json << 'ENDTOKEN'
{
  "grant": "broker",
  "brokerUrl": "http://localhost:8799",
  "provider": "google",
  "accessToken": "x",
  "refreshToken": "bad",
  "expiresAt": "2020-01-01T00:00:00Z"
}
ENDTOKEN
```

```execute
aux4 curl oauth token brokerbad
```

```error:partial
broker refresh failed: *?400
```
