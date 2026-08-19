# curl auth from config

## definitions read from config.yaml

```timeout
30000
```

```beforeAll
# A fresh CI runner has only this package and its declared dependencies, so the
# tools these tests drive are installed here. Doing it in the test rather than
# in the workflow keeps the suite self-contained and identical locally.
aux4 aux4 pkger list 2>/dev/null | grep -q "aux4/secret-aux4" || aux4 aux4 pkger install aux4/secret-aux4 > /dev/null 2>&1
cat > /tmp/aux4-curl-cfg-18955.js << 'ENDJS'
const http = require('http');
let issued = 0;
http.createServer((req, res) => {
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    const raw = Buffer.concat(chunks).toString();
    const json = (code, obj) => {
      res.writeHead(code, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(obj));
    };
    if (req.url === '/ping') return json(200, { ok: true });
    if (req.url === '/issued') { res.writeHead(200, { 'Content-Type': 'text/plain' }); return res.end(String(issued)); }
    if (req.url === '/token') {
      const p = new URLSearchParams(raw);
      if (p.get('client_id') !== 'id1' || p.get('client_secret') !== 'sec1') {
        return json(401, { error: 'invalid_client', error_description: 'bad credentials' });
      }
      issued++;
      return json(200, { access_token: 'tok-' + issued, expires_in: 3600 });
    }
    if (req.url.startsWith('/api')) {
      return json(200, { auth: req.headers['authorization'] || '' });
    }
    json(404, { error: 'not found' });
  });
}).listen(18955, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-cfg-18955.js >/dev/null 2>&1 &
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18955/ping 2>/dev/null && break; sleep 0.25; done

cat > config.yaml << 'ENDYAML'
config:
  prod:
    auth:
      billing:
        tokenUrl: http://127.0.0.1:18955/token
        preset: client-credentials
        clientId: id1
        clientSecret: sec1
        scopes: invoices:read
      missingsecret:
        tokenUrl: http://127.0.0.1:18955/token
        preset: client-credentials
        clientId: id1
        clientSecretRef: nosuchprovider/Work/Item/field
ENDYAML
```

```afterAll
pkill -f aux4-curl-cfg-18955.js
rm -f /tmp/aux4-curl-cfg-18955.js
rm -rf .oauth config.yaml written.yaml
```

### should authenticate with no prior auth add

The definition comes from the shared config, so a fresh checkout needs no
setup step: the token is fetched on first use.

```execute
rm -rf .oauth
aux4 curl auth-request --provider billing --configFile config.yaml --config prod http://127.0.0.1:18955/api
```

```expect:json
{
  "auth": "Bearer tok-1"
}
```

### should cache the token locally after the first call

```execute
rm -rf .oauth
aux4 curl auth-request --provider billing --configFile config.yaml --config prod http://127.0.0.1:18955/api > /dev/null
before=$(aux4 curl request http://127.0.0.1:18955/issued)
aux4 curl auth-request --provider billing --configFile config.yaml --config prod http://127.0.0.1:18955/api > /dev/null
after=$(aux4 curl request http://127.0.0.1:18955/issued)
echo "extra fetches: $((after - before))"
```

```expect
extra fetches: 0
```

### should read the token through auth token as well

```execute
rm -rf .oauth
aux4 curl auth token billing --configFile config.yaml --config prod
```

```expect:partial
tok-
```

### should report an auth name that is not in the configuration

```execute
rm -rf .oauth
aux4 curl auth-request --provider nosuchname --configFile config.yaml --config prod http://127.0.0.1:18955/api
```

```error:partial
no auth named "nosuchname" in the configuration
```

### should resolve a secret reference rather than use it literally

The reference names a provider that is not installed, which proves the value
is being resolved through the secret provider and not passed through as if it
were the secret itself.

```execute
rm -rf .oauth
aux4 curl auth-request --provider missingsecret --configFile config.yaml --config prod http://127.0.0.1:18955/api
```

```error:partial
no secret provider "nosuchprovider" is installed
```

## writing a definition to config

```timeout
30000
```

```beforeAll
# A fresh CI runner has only this package and its declared dependencies, so the
# tools these tests drive are installed here. Doing it in the test rather than
# in the workflow keeps the suite self-contained and identical locally.
aux4 aux4 pkger list 2>/dev/null | grep -q "aux4/secret-aux4" || aux4 aux4 pkger install aux4/secret-aux4 > /dev/null 2>&1
aux4 secret aux4 create --vault Aux4CurlTest --item Billing --fields "clientSecret=sec1" > /dev/null 2>&1
cat > /tmp/aux4-curl-cfgw-18956.js << 'ENDJS'
const http = require('http');
http.createServer((req, res) => {
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    const json = (code, obj) => {
      res.writeHead(code, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(obj));
    };
    if (req.url === '/ping') return json(200, { ok: true });
    if (req.url === '/token') return json(200, { access_token: 'tok-x', expires_in: 3600 });
    json(404, { error: 'not found' });
  });
}).listen(18956, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-cfgw-18956.js >/dev/null 2>&1 &
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18956/ping 2>/dev/null && break; sleep 0.25; done
```

```afterAll
aux4 secret aux4 remove --ref Aux4CurlTest/Billing > /dev/null 2>&1 || true
pkill -f aux4-curl-cfgw-18956.js
rm -f /tmp/aux4-curl-cfgw-18956.js
rm -rf .oauth written.yaml
```

### should write the definition under the config profile

```execute
rm -f written.yaml
printf 'config:\n  prod:\n    host: api.example.com\n' > written.yaml
aux4 curl auth add billing --tokenUrl http://127.0.0.1:18956/token --clientId id1 --clientSecretRef 'aux4/Aux4CurlTest/Billing/clientSecret' --configFile written.yaml --config prod --save true --verify false 2>&1
aux4 config get --file written.yaml prod | grep -o '"tokenUrl":"[^"]*"' | head -1
```

```expect
Auth "billing" saved to written.yaml
"tokenUrl":"http://127.0.0.1:18956/token"
```

### should keep the rest of the config file intact

```execute
rm -f written.yaml
printf 'config:\n  prod:\n    host: api.example.com\n' > written.yaml
aux4 curl auth add billing --tokenUrl http://127.0.0.1:18956/token --clientId id1 --clientSecretRef 'aux4/Aux4CurlTest/Billing/clientSecret' --configFile written.yaml --config prod --save true --verify false > /dev/null 2>&1
aux4 config get --file written.yaml prod | grep -o '"host":"[^"]*"'
```

```expect
"host":"api.example.com"
```

### should store the reference and never the secret itself

```execute
rm -f written.yaml
printf 'config:\n  prod:\n    host: api.example.com\n' > written.yaml
aux4 curl auth add billing --tokenUrl http://127.0.0.1:18956/token --clientId id1 --clientSecretRef 'aux4/Aux4CurlTest/Billing/clientSecret' --configFile written.yaml --config prod --save true --verify false > /dev/null 2>&1
aux4 config get --file written.yaml prod | grep -o '"clientSecretRef":"[^"]*"'
grep -c "clientSecret:" written.yaml || true
```

```expect
"clientSecretRef":"aux4/Aux4CurlTest/Billing/clientSecret"
0
```

### should refuse to write an inline secret into a shareable file

```execute
rm -f written.yaml
printf 'config:\n  prod:\n    host: api.example.com\n' > written.yaml
aux4 curl auth add leaky --tokenUrl http://127.0.0.1:18956/token --clientId id1 --clientSecret s3cr3t --configFile written.yaml --config prod --save true --verify false
```

```error:partial
refusing to save an inline clientSecret
```

### should reject a secret given both inline and by reference

```execute
aux4 curl auth add both --tokenUrl http://127.0.0.1:18956/token --clientSecret a --clientSecretRef b --verify false
```

```error:partial
use either clientSecret or clientSecretRef, not both
```

## keeping the secret out of stored files

```timeout
30000
```

```beforeAll
# A fresh CI runner has only this package and its declared dependencies, so the
# tools these tests drive are installed here. Doing it in the test rather than
# in the workflow keeps the suite self-contained and identical locally.
aux4 aux4 pkger list 2>/dev/null | grep -q "aux4/secret-aux4" || aux4 aux4 pkger install aux4/secret-aux4 > /dev/null 2>&1
cat > /tmp/aux4-curl-leak-18957.js << 'ENDJS'
const http = require('http');
http.createServer((req, res) => {
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    const json = (code, obj) => {
      res.writeHead(code, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(obj));
    };
    if (req.url === '/ping') return json(200, { ok: true });
    if (req.url === '/token') return json(200, { access_token: 'tok-x', expires_in: 3600 });
    json(404, { error: 'not found' });
  });
}).listen(18957, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-leak-18957.js >/dev/null 2>&1 &
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18957/ping 2>/dev/null && break; sleep 0.25; done
```

```afterAll
pkill -f aux4-curl-leak-18957.js
rm -f /tmp/aux4-curl-leak-18957.js
rm -rf .oauth leak.yaml
```

### should reject a reference given as a secret:// uri

aux4 resolves a parameter whose whole value is a `secret://` URI before the
command runs, so what would arrive here is the secret itself. Writing that out
as though it were a reference would put the secret into a file meant to be
committed.

```execute
rm -f leak.yaml
printf 'config:\n  prod:\n    host: api.example.com\n' > leak.yaml
aux4 curl auth add billing --tokenUrl http://127.0.0.1:18957/token --clientSecretRef 'a-resolved-secret-value' --configFile leak.yaml --config prod --save true --verify false
```

```error:partial
the secret reference must be <provider>/<vault>/<item>/<field>
```

### should not echo the offending value

If that check is firing, the value is most likely the secret itself, and
repeating it would put it into terminal scrollback and CI logs.

```execute
aux4 curl auth add billing --tokenUrl http://127.0.0.1:18957/token --clientSecretRef 'a-resolved-secret-value' --verify false 2>&1 | grep -c "a-resolved-secret-value" || true
```

```expect
0
```

### should reject a reference that resolves to nothing

```execute
aux4 curl auth add billing --tokenUrl http://127.0.0.1:18957/token --clientSecretRef 'aux4/NoSuchVault/NoSuchItem/field' --verify false
```

```error:partial
The value must name a secret, not be one
```

### should report an absent provider without inventing a package name

The provider in a reference is a command name; nothing about it reveals the
scope or package behind it. Guessing one would hand the user an install command
that is very likely wrong, so the error states only what can be observed.

```execute
aux4 curl auth add x --tokenUrl http://127.0.0.1:18957/token --clientSecretRef 'onepassword/Work/API/pw' --verify false
```

```error:partial
no secret provider "onepassword" is installed
```

### should not suggest a package to install

```execute
aux4 curl auth add x --tokenUrl http://127.0.0.1:18957/token --clientSecretRef 'onepassword/Work/API/pw' --verify false 2>&1 | grep -c "pkger install" || true
```

```expect
0
```

### should not add advice about the value when the provider is missing

The reference may be perfectly correct; the problem is setup, so pointing at
the value would send the user after the wrong thing.

```execute
aux4 curl auth add x --tokenUrl http://127.0.0.1:18957/token --clientSecretRef 'onepassword/Work/API/pw' --verify false 2>&1 | grep -c "must name a secret" || true
```

```expect
0
```
