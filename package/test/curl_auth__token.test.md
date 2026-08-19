# curl auth token

## caching and renewal

```timeout
30000
```

```beforeAll
# A fresh CI runner has only this package and its declared dependencies, so the
# tools these tests drive are installed here. Doing it in the test rather than
# in the workflow keeps the suite self-contained and identical locally.
aux4 aux4 pkger list 2>/dev/null | grep -q "aux4/json" || aux4 aux4 pkger install aux4/json > /dev/null 2>&1
cat > /tmp/aux4-curl-authtok-18953.js << 'ENDJS'
const http = require('http');
let issued = 0;
let issuedShort = 0;
http.createServer((req, res) => {
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    const json = (code, obj) => {
      res.writeHead(code, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(obj));
    };
    if (req.url === '/ping') return json(200, { ok: true });
    if (req.url === '/issued') return json(200, { issued: issued, short: issuedShort });
    if (req.url === '/token') {
      issued++;
      return json(200, { access_token: 'tok-' + issued, expires_in: 3600 });
    }
    // Expires inside the refresh skew, so it is due for renewal on arrival.
    if (req.url === '/short') {
      issuedShort++;
      return json(200, { access_token: 'short-' + issuedShort, expires_in: 1 });
    }
    json(404, { error: 'not found' });
  });
}).listen(18953, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-authtok-18953.js >/dev/null 2>&1 &
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18953/ping 2>/dev/null && break; sleep 0.25; done
```

```afterAll
pkill -f aux4-curl-authtok-18953.js
rm -f /tmp/aux4-curl-authtok-18953.js
rm -rf .oauth
```

### should reuse the stored token while it is still valid

Three reads, one token: the endpoint is called once at registration and not
again while the cached token is inside its lifetime.

```execute
before=$(aux4 curl request http://127.0.0.1:18953/issued | aux4 json get '$.issued')
aux4 curl auth add cached --tokenUrl http://127.0.0.1:18953/token >/dev/null 2>&1
aux4 curl auth token cached
aux4 curl auth token cached
aux4 curl auth token cached
echo ""
after=$(aux4 curl request http://127.0.0.1:18953/issued | aux4 json get '$.issued')
echo "fetches: $((after - before))"
```

```expect:partial
fetches: 1
```

### should defer the first fetch until the token is used

`--verify false` stores the recipe without calling the endpoint, so nothing is
requested until something actually needs a token.

```execute
before=$(aux4 curl request http://127.0.0.1:18953/issued | aux4 json get '$.issued')
aux4 curl auth add lazy --tokenUrl http://127.0.0.1:18953/token --verify false >/dev/null 2>&1
middle=$(aux4 curl request http://127.0.0.1:18953/issued | aux4 json get '$.issued')
aux4 curl auth token lazy >/dev/null 2>&1
after=$(aux4 curl request http://127.0.0.1:18953/issued | aux4 json get '$.issued')
echo "on add: $((middle - before))  on first use: $((after - middle))"
```

```expect
on add: 0  on first use: 1
```

### should fetch again once the stored token has expired

```execute
before=$(aux4 curl request http://127.0.0.1:18953/issued | aux4 json get '$.short')
aux4 curl auth add shortlived --tokenUrl http://127.0.0.1:18953/short --verify false >/dev/null 2>&1
aux4 curl auth token shortlived >/dev/null 2>&1
aux4 curl auth token shortlived >/dev/null 2>&1
after=$(aux4 curl request http://127.0.0.1:18953/issued | aux4 json get '$.short')
echo "fetches: $((after - before))"
```

```expect
fetches: 2
```

### should renew concurrent callers with a single fetch

Five processes each find no usable token. The file lock means one of them
fetches and the rest reuse that result.

```execute
aux4 curl auth add shared --tokenUrl http://127.0.0.1:18953/token --verify false >/dev/null 2>&1
before=$(aux4 curl request http://127.0.0.1:18953/issued | aux4 json get '$.issued')
for i in 1 2 3 4 5; do aux4 curl auth token shared >/dev/null 2>&1 & done
wait
after=$(aux4 curl request http://127.0.0.1:18953/issued | aux4 json get '$.issued')
echo "fetches: $((after - before))"
```

```expect
fetches: 1
```

## with an unknown name

### should show error

```execute
aux4 curl auth token nosuchauth
```

```error:partial
no token found for provider "nosuchauth"
```
