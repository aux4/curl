# curl auth-request

## with valid token

```timeout
20000
```

```beforeAll
nohup python3 -c "
from http.server import HTTPServer, BaseHTTPRequestHandler
import json

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        auth = self.headers.get('Authorization', '')
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        response = json.dumps({'auth': auth, 'path': self.path})
        self.wfile.write(response.encode())
    def do_POST(self):
        length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(length) if length else b''
        auth = self.headers.get('Authorization', '')
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        response = json.dumps({'auth': auth, 'body': json.loads(body) if body else None})
        self.wfile.write(response.encode())
    def log_message(self, format, *args):
        pass

HTTPServer(('', 18931), Handler).serve_forever()
" >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-auth-request-server.pid
for i in $(seq 1 12); do curl -s -o /dev/null --noproxy '*' --connect-timeout 1 --max-time 1 http://127.0.0.1:18931/ 2>/dev/null && break; sleep 0.25; done

mkdir -p .oauth
cat > .oauth/testprovider.json << 'ENDTOKEN'
{
  "clientId": "test-client",
  "clientSecret": "test-secret",
  "authUrl": "http://localhost/auth",
  "tokenUrl": "http://localhost/token",
  "scopes": "read,write",
  "accessToken": "my-secret-token",
  "refreshToken": "my-refresh-token",
  "expiresAt": "2099-12-31T23:59:59Z"
}
ENDTOKEN
```

```afterAll
kill $(cat /tmp/aux4-curl-auth-request-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-auth-request-server.pid
rm -rf .oauth
```

### should inject authorization header on GET

```execute
aux4 curl auth-request --provider testprovider http://localhost:18931/test
```

```expect:json
{
  "auth": "Bearer my-secret-token",
  "path": "/test"
}
```

### should inject authorization header on POST

```execute
aux4 curl auth-request --provider testprovider --method POST --body '{"name":"test"}' --header "Content-Type: application/json" http://localhost:18931/data
```

```expect:json
{
  "auth": "Bearer my-secret-token",
  "body": {
    "name": "test"
  }
}
```

## with missing provider

### should show error

```execute
aux4 curl auth-request --provider nonexistent http://localhost:18931/test
```

```error:partial
no token found for provider "nonexistent"
```
