# curl auth-stream

## with valid token

```beforeAll
nohup python3 -c "
from http.server import HTTPServer, BaseHTTPRequestHandler
import json

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(length) if length else b''
        auth = self.headers.get('Authorization', '')
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        response = json.dumps({'auth': auth, 'received': json.loads(body) if body else None})
        self.wfile.write(response.encode())
    def log_message(self, format, *args):
        pass

HTTPServer(('', 18932), Handler).serve_forever()
" >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-auth-stream-server.pid
sleep 3

mkdir -p .oauth
cat > .oauth/testprovider.json << 'ENDTOKEN'
{
  "clientId": "test-client",
  "clientSecret": "test-secret",
  "authUrl": "http://localhost/auth",
  "tokenUrl": "http://localhost/token",
  "scopes": "read,write",
  "accessToken": "stream-token-123",
  "refreshToken": "my-refresh-token",
  "expiresAt": "2099-12-31T23:59:59Z"
}
ENDTOKEN
```

```afterAll
kill $(cat /tmp/aux4-curl-auth-stream-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-auth-stream-server.pid
rm -rf .oauth
```

### should stream with auth header

```execute
echo '{"id":1}' | aux4 curl auth-stream --provider testprovider --header "Content-Type: application/json" http://localhost:18932/
```

```expect:json:partial
{
  "body": {
    "auth": "Bearer stream-token-123",
    "received": {
      "id": 1
    }
  },
  "input": {
    "id": 1
  },
  "status": 200
}
```
