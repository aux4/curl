# curl request

## with a GET request

```beforeAll
nohup python3 -m http.server 18923 --directory /tmp >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-test-server.pid
sleep 3
echo "test-content" > /tmp/test-file.txt
```

```afterAll
kill $(cat /tmp/aux4-curl-test-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-test-server.pid /tmp/test-file.txt
```

### should fetch a resource

```execute
aux4 curl request http://localhost:18923/test-file.txt
```

```expect
test-content
```

### should show response headers

```execute
aux4 curl request --showHeaders true http://localhost:18923/test-file.txt
```

```expect:partial
HTTP/1.0 200 OK
**
test-content
```

## with a POST request

```beforeAll
nohup python3 -c "
from http.server import HTTPServer, BaseHTTPRequestHandler
import json

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(length) if length else b''
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        response = json.dumps({'received': json.loads(body) if body else None})
        self.wfile.write(response.encode())
    def log_message(self, format, *args):
        pass

HTTPServer(('', 18924), Handler).serve_forever()
" >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-test-post-server.pid
sleep 3
```

```afterAll
kill $(cat /tmp/aux4-curl-test-post-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-test-post-server.pid
```

### should send body from flag

```execute
aux4 curl request --method POST --body '{"name":"Alice"}' --header "Content-Type: application/json" http://localhost:18924/
```

```expect:json
{
  "received": {
    "name": "Alice"
  }
}
```

### should send body with custom header

```execute
aux4 curl request --method POST --body '{"greeting":"hello"}' --header "Content-Type: application/json" http://localhost:18924/
```

```expect:json
{
  "received": {
    "greeting": "hello"
  }
}
```
