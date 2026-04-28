# curl stream

```beforeAll
nohup python3 -c "
from http.server import HTTPServer, BaseHTTPRequestHandler
import json

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(length) if length else b''
        data = json.loads(body)
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        response = json.dumps({'processed': True, 'id': data.get('id')})
        self.wfile.write(response.encode())
    def log_message(self, format, *args):
        pass

HTTPServer(('', 18925), Handler).serve_forever()
" >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-test-stream-server.pid
sleep 1
```

```afterAll
kill $(cat /tmp/aux4-curl-test-stream-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-test-stream-server.pid
```

## with NDJSON input

### should process single JSON line

```execute
echo '{"id":1}' | aux4 curl stream http://localhost:18925/
```

```expect:json
{
  "body": {
    "processed": true,
    "id": 1
  },
  "input": {
    "id": 1
  },
  "status": 200
}
```

### should process multiple JSON lines

```execute
printf '{"id":1}\n{"id":2}\n' | aux4 curl stream http://localhost:18925/
```

```expect:partial
*"id":1*
*"id":2*
```

### should skip empty lines

```execute
printf '\n{"id":3}\n\n' | aux4 curl stream http://localhost:18925/
```

```expect:json
{
  "body": {
    "processed": true,
    "id": 3
  },
  "input": {
    "id": 3
  },
  "status": 200
}
```

### should report invalid JSON on stderr

```execute
printf 'not-json\n{"id":4}\n' | aux4 curl stream http://localhost:18925/
```

```error:partial
Error: invalid JSON*
```

## with concurrency

### should handle concurrent requests

```execute
printf '{"id":10}\n{"id":11}\n{"id":12}\n' | aux4 curl stream --concurrency 3 http://localhost:18925/ | sort
```

```expect:partial
*"id":10*
*"id":11*
*"id":12*
```
