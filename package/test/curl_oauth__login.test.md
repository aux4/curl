# curl oauth login

## login and token exchange

```timeout
20000
```

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

HTTPServer(('127.0.0.1', 18930), Handler).serve_forever()
" >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-oauth-server.pid
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18930/ 2>/dev/null && break; sleep 0.25; done
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
  "authUrl": "http://127.0.0.1:18930/auth",
  "tokenUrl": "http://127.0.0.1:18930/token",
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

## authorize url construction

### should append params with & when authUrl already has a query string

```execute
aux4 curl oauth login gtestprov --clientId CID --clientSecret "" --authUrl "https://accounts.example.com/auth?access_type=offline&prompt=consent" --tokenUrl https://accounts.example.com/token --scopes "openid email" --callbackPort 18939 --tokenFile /tmp/aux4-authurl-token.json >/dev/null 2>/tmp/aux4-authurl.txt &
LOGIN_PID=$!
sleep 2
kill $LOGIN_PID 2>/dev/null
wait $LOGIN_PID 2>/dev/null
grep -oE "auth\?access_type=offline&prompt=consent&client_id=CID" /tmp/aux4-authurl.txt | head -1
rm -f /tmp/aux4-authurl.txt /tmp/aux4-authurl-token.json
```

```expect
auth?access_type=offline&prompt=consent&client_id=CID
```
