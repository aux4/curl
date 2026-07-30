# curl stream

```timeout
20000
```

```beforeAll
cat > /tmp/aux4-curl-srv-18925.js << 'ENDJS'
const http = require('http');
http.createServer((req, res) => {
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    let data = {};
    try { data = JSON.parse(Buffer.concat(chunks).toString()); } catch (e) { data = {}; }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ processed: true, id: data.id !== undefined ? data.id : null }));
  });
}).listen(18925, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-srv-18925.js >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-test-stream-server.pid
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18925/ 2>/dev/null && break; sleep 0.25; done
```

```afterAll
kill $(cat /tmp/aux4-curl-test-stream-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-test-stream-server.pid
```

## with NDJSON input

### should process single JSON line

```execute
echo '{"id":1}' | aux4 curl stream http://127.0.0.1:18925/
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
printf '{"id":1}\n{"id":2}\n' | aux4 curl stream http://127.0.0.1:18925/
```

```expect:partial
*"id":1*
*"id":2*
```

### should skip empty lines

```execute
printf '\n{"id":3}\n\n' | aux4 curl stream http://127.0.0.1:18925/
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
printf 'not-json\n{"id":4}\n' | aux4 curl stream http://127.0.0.1:18925/
```

```error:partial
Error: invalid JSON*
```

## with concurrency

### should handle concurrent requests

```execute
printf '{"id":10}\n{"id":11}\n{"id":12}\n' | aux4 curl stream --concurrency 3 http://127.0.0.1:18925/ | sort
```

```expect:partial
*"id":10*
*"id":11*
*"id":12*
```
