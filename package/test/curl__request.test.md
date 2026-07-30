# curl request

## with a GET request

```timeout
20000
```

```beforeAll
echo "DIAG python3: $(command -v python3) [$(python3 --version 2>&1)]"
echo "DIAG curl: $(curl --version 2>&1 | head -1)"
echo "DIAG uname: $(uname -a)"
nohup python3 -m http.server 18923 --directory /tmp > /tmp/aux4-curl-srv-18923.log 2>&1 &
SRVPID=$!
echo $SRVPID > /tmp/aux4-curl-test-server.pid
for i in $(seq 1 12); do curl -s -o /dev/null --noproxy '*' --connect-timeout 1 --max-time 1 http://127.0.0.1:18923/ 2>/dev/null && break; sleep 0.25; done
if ! curl -s -o /dev/null --noproxy '*' --max-time 1 http://127.0.0.1:18923/ 2>/dev/null; then
  echo "DIAG ### 18923 UNREACHABLE after poll ###"
  echo "DIAG server pid $SRVPID alive: $(kill -0 $SRVPID 2>/dev/null && echo yes || echo no)"
  echo "DIAG server log:"; cat /tmp/aux4-curl-srv-18923.log 2>/dev/null | sed 's/^/DIAG   /'
  echo "DIAG listening on 18923:"; (lsof -nP -iTCP:18923 2>/dev/null || netstat -an 2>/dev/null | grep 18923 || echo "  (nothing)") | sed 's/^/DIAG   /'
  echo "DIAG loopback GET localhost:"; curl -s -o /dev/null -w "DIAG   localhost -> %{http_code} (exit varies)\n" --max-time 1 http://localhost:18923/ 2>&1 || echo "DIAG   localhost unreachable"
fi
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

```timeout
20000
```

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
for i in $(seq 1 12); do curl -s -o /dev/null --noproxy '*' --connect-timeout 1 --max-time 1 http://127.0.0.1:18924/ 2>/dev/null && break; sleep 0.25; done
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

### should send a file as the raw request body

```execute
printf '{"name":"Alice"}' > /tmp/aux4-curl-bodyfile.json
aux4 curl request --method POST --bodyFile /tmp/aux4-curl-bodyfile.json --header "Content-Type: application/json" http://localhost:18924/submit
rm -f /tmp/aux4-curl-bodyfile.json
```

```expect
{"received": {"name": "Alice"}}
```


## with a file upload

```timeout
20000
```

```beforeAll
nohup python3 -c "
from http.server import HTTPServer, BaseHTTPRequestHandler
from email.parser import BytesParser
from email.policy import default
import json, threading, os
threading.Timer(90, lambda: os._exit(0)).start()

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        ctype = self.headers.get('Content-Type', '')
        raw = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        if self.path == '/ctype':
            self.send_response(200)
            self.send_header('Content-Type', 'text/plain')
            self.end_headers()
            self.wfile.write((ctype.split(';')[0] + '\n').encode())
            return
        msg = BytesParser(policy=default).parsebytes(b'Content-Type: ' + ctype.encode() + b'\r\n\r\n' + raw)
        parts = []
        for part in msg.iter_parts():
            name = part.get_param('name', header='content-disposition')
            filename = part.get_filename()
            payload = part.get_payload(decode=True) or b''
            if filename:
                parts.append(name + ':' + filename + ':' + str(len(payload)))
            else:
                parts.append(name + '=' + payload.decode())
        self.send_response(200)
        self.send_header('Content-Type', 'text/plain')
        self.end_headers()
        self.wfile.write((' '.join(parts) + '\n').encode())
    def do_GET(self):
        if self.path == '/ctype':
            self.send_response(200)
            self.send_header('Content-Type', 'text/plain')
            self.end_headers()
            self.wfile.write((self.headers.get('Content-Type', '').split(';')[0] + '\n').encode())
            return
        if self.path == '/headers':
            names = sorted(k + ': ' + v for k, v in self.headers.items() if k.lower().startswith('x-'))
            self.send_response(200)
            self.send_header('Content-Type', 'text/plain')
            self.end_headers()
            self.wfile.write((' | '.join(names) + '\n').encode())
            return
        self.send_response(200)
        self.send_header('Content-Type', 'application/octet-stream')
        self.end_headers()
        self.wfile.write(bytes(range(256)))
    def log_message(self, format, *args):
        pass

HTTPServer(('', 18933), Handler).serve_forever()
" >/dev/null 2>&1 &
for i in $(seq 1 12); do curl -s -o /dev/null --noproxy '*' --connect-timeout 1 --max-time 1 http://127.0.0.1:18933/ 2>/dev/null && break; sleep 0.25; done
printf 'aaa\n' > /tmp/aux4-curl-up-a.txt
printf 'bbbbb\n' > /tmp/aux4-curl-up-b.txt
```

```afterAll
pkill -f "18933" 2>/dev/null
rm -f /tmp/aux4-curl-up-a.txt /tmp/aux4-curl-up-b.txt /tmp/aux4-curl-out.bin
```

### should upload a file as multipart form data

```execute
aux4 curl request --method POST --upload /tmp/aux4-curl-up-a.txt --uploadField media http://localhost:18933/upload
```

```expect
media:aux4-curl-up-a.txt:4
```

### should send body entries as extra form fields

```execute
aux4 curl request --method POST --upload /tmp/aux4-curl-up-a.txt --uploadField media --body '{"media_category":"tweet_image"}' http://localhost:18933/upload
```

```expect
media_category=tweet_image media:aux4-curl-up-a.txt:4
```

### should upload multiple files under the same field

```execute
aux4 curl request --method POST --upload /tmp/aux4-curl-up-a.txt --upload /tmp/aux4-curl-up-b.txt --uploadField files http://localhost:18933/upload
```

```expect
files:aux4-curl-up-a.txt:4 files:aux4-curl-up-b.txt:6
```

### should name each part with field=path

```execute
aux4 curl request --method POST --upload avatar=/tmp/aux4-curl-up-a.txt --upload banner=/tmp/aux4-curl-up-b.txt http://localhost:18933/upload
```

```expect
avatar:aux4-curl-up-a.txt:4 banner:aux4-curl-up-b.txt:6
```

### should write a binary response to a file

```execute
aux4 curl request --output /tmp/aux4-curl-out.bin http://localhost:18933/binary < /dev/null
wc -c < /tmp/aux4-curl-out.bin | tr -d ' '
```

```expect
256
```

### should send every repeated header

```execute
aux4 curl request --header "X-One: 1" --header "X-Two: 2" --header "X-Three: 3" http://localhost:18933/headers < /dev/null
```

```expect
X-One: 1 | X-Three: 3 | X-Two: 2
```

### should default the multipart subtype to form-data

```execute
aux4 curl request --method POST --upload /tmp/aux4-curl-up-a.txt http://localhost:18933/ctype
```

```expect
multipart/form-data
```

### should keep a caller-chosen multipart subtype

```execute
aux4 curl request --method POST --upload /tmp/aux4-curl-up-a.txt --header "Content-Type: multipart/related" http://localhost:18933/ctype
```

```expect
multipart/related
```
