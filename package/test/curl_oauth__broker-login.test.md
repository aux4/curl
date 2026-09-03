# curl oauth login --broker

The `--broker` mode delegates the client secret to an OAuth broker: the broker
builds the authorization URL (returning the PKCE verifier) and performs the code
exchange and later the refresh, so this client never handles a secret. These
tests stand up a tiny local Node mock broker (`mock-broker-server.js`) that mimics
`aux4/oauth-broker`'s three routes on `localhost:8799`, and drive the loopback
callback with `curl` in place of a real browser.

```beforeAll
STATE_FILE=state.txt nohup node mock-broker-server.js >/dev/null 2>&1 &
sleep 1
mkdir -p .oauth
```

```afterAll
pkill -f mock-broker-server.js
rm -rf .oauth state.txt login.log
```

## broker login (loopback)

### should obtain and store a broker-grant token via the loopback flow

```execute
rm -f state.txt .oauth/google.json
aux4 curl oauth login google --scopes "openid email" --callbackPort 9878 --broker http://localhost:8799 --tokenFile .oauth/google.json >login.log 2>&1 &
for i in $(seq 1 30); do [ -s state.txt ] && break; sleep 0.2; done
curl -s "http://localhost:9878/callback?code=good&state=$(cat state.txt)" >/dev/null
for i in $(seq 1 30); do grep -q accessToken .oauth/google.json 2>/dev/null && break; sleep 0.2; done
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
