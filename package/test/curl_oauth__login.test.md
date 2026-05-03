# curl oauth login

## login and token exchange

```beforeAll
nohup python3 -c "
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs
import json

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(length).decode()
        params = dict(p.split('=') for p in body.split('&'))
        if params.get('grant_type') == 'authorization_code':
            response = json.dumps({
                'access_token': 'test-access-token',
                'refresh_token': 'test-refresh-token',
                'expires_in': 3600
            })
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(response.encode())
        else:
            self.send_response(400)
            self.end_headers()
    def log_message(self, format, *args):
        pass

HTTPServer(('', 18930), Handler).serve_forever()
" >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-oauth-server.pid
sleep 3
```

```afterAll
kill $(cat /tmp/aux4-curl-oauth-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-oauth-server.pid
rm -rf .oauth
```

### should save token after login

```execute
mkdir -p .oauth
cat > .oauth/testprovider.json << 'ENDTOKEN'
{
  "clientId": "test-client",
  "clientSecret": "test-secret",
  "authUrl": "http://localhost:18930/auth",
  "tokenUrl": "http://localhost:18930/token",
  "scopes": "read,write",
  "accessToken": "test-access-token",
  "refreshToken": "test-refresh-token",
  "expiresAt": "2099-12-31T23:59:59Z"
}
ENDTOKEN
aux4 curl oauth status testprovider
```

```expect:partial
Provider:      testprovider
Status:        valid
Scopes:        read,write
```
