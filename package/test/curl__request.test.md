# curl request

## with a GET request

```timeout
20000
```

```beforeAll
cat > /tmp/aux4-curl-srv-18923.js << 'ENDJS'
const http = require('http'), fs = require('fs'), path = require('path');
http.createServer((req, res) => {
  const f = path.join('/tmp', decodeURIComponent(req.url.split('?')[0]));
  fs.readFile(f, (e, d) => {
    if (e) { res.writeHead(404); res.end('not found\n'); return; }
    res.writeHead(200, { 'Content-Type': 'application/octet-stream' });
    res.end(d);
  });
}).listen(18923, '127.0.0.1');
ENDJS
echo "test-content" > /tmp/test-file.txt
nohup node /tmp/aux4-curl-srv-18923.js >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-test-server.pid
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18923/test-file.txt 2>/dev/null && break; sleep 0.25; done
```

```afterAll
kill $(cat /tmp/aux4-curl-test-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-test-server.pid /tmp/test-file.txt
```

### should fetch a resource

```execute
aux4 curl request http://127.0.0.1:18923/test-file.txt
```

```expect
test-content
```

### should show response headers

```execute
aux4 curl request --showHeaders true http://127.0.0.1:18923/test-file.txt
```

```expect:partial
HTTP/1.1 200 OK
**
test-content
```

## with a POST request

```timeout
20000
```

```beforeAll
cat > /tmp/aux4-curl-srv-18924.js << 'ENDJS'
const http = require('http');
http.createServer((req, res) => {
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', () => {
    const body = Buffer.concat(chunks).toString();
    let received = null;
    try { received = body ? JSON.parse(body) : null; } catch (e) { received = null; }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ received: received }));
  });
}).listen(18924, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-srv-18924.js >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-test-post-server.pid
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18924/ 2>/dev/null && break; sleep 0.25; done
```

```afterAll
kill $(cat /tmp/aux4-curl-test-post-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-test-post-server.pid
```

### should send body from flag

```execute
aux4 curl request --method POST --body '{"name":"Alice"}' --header "Content-Type: application/json" http://127.0.0.1:18924/
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
aux4 curl request --method POST --body '{"greeting":"hello"}' --header "Content-Type: application/json" http://127.0.0.1:18924/
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
aux4 curl request --method POST --bodyFile /tmp/aux4-curl-bodyfile.json --header "Content-Type: application/json" http://127.0.0.1:18924/submit
rm -f /tmp/aux4-curl-bodyfile.json
```

```expect
{"received":{"name":"Alice"}}
```


## with a file upload

```timeout
20000
```

```beforeAll
cat > /tmp/aux4-curl-srv-18933.js << 'ENDJS'
const http = require('http');
setTimeout(() => process.exit(0), 90000);

function splitBuffer(buf, sep) {
  const parts = [];
  let start = 0, idx;
  while ((idx = buf.indexOf(sep, start)) !== -1) {
    parts.push(buf.slice(start, idx));
    start = idx + sep.length;
  }
  parts.push(buf.slice(start));
  return parts;
}

http.createServer((req, res) => {
  const ctype = req.headers['content-type'] || '';
  if (req.method === 'POST') {
    const chunks = [];
    req.on('data', c => chunks.push(c));
    req.on('end', () => {
      if (req.url === '/ctype') {
        res.writeHead(200, { 'Content-Type': 'text/plain' });
        res.end(ctype.split(';')[0] + '\n');
        return;
      }
      if (req.url === '/related') {
        // multipart/related parts carry Content-Type, not Content-Disposition.
        const rbm = ctype.match(/boundary=([^;]+)/);
        const rboundary = rbm ? rbm[1].trim() : '';
        const rbody = Buffer.concat(chunks);
        const parts = [];
        for (const p of splitBuffer(rbody, Buffer.from('--' + rboundary))) {
          const he = p.indexOf('\r\n\r\n');
          if (he < 0) continue;
          const head = p.slice(0, he).toString();
          let content = p.slice(he + 4);
          if (content.length >= 2 && content.slice(-2).toString() === '\r\n') content = content.slice(0, -2);
          const cm = head.match(/Content-Type:\s*([^\r\n]+)/i);
          if (!cm) continue;
          parts.push(cm[1].trim() + '=' + content.toString().trim());
        }
        res.writeHead(200, { 'Content-Type': 'text/plain' });
        res.end(parts.join(' | ') + '\n');
        return;
      }
      const bm = ctype.match(/boundary=([^;]+)/);
      const boundary = bm ? bm[1].trim() : '';
      const body = Buffer.concat(chunks);
      const out = [];
      for (const p of splitBuffer(body, Buffer.from('--' + boundary))) {
        const he = p.indexOf('\r\n\r\n');
        if (he < 0) continue;
        const head = p.slice(0, he).toString();
        let content = p.slice(he + 4);
        if (content.length >= 2 && content.slice(-2).toString() === '\r\n') content = content.slice(0, -2);
        const nm = head.match(/name="([^"]*)"/);
        if (!nm) continue;
        const fm = head.match(/filename="([^"]*)"/);
        if (fm) out.push(nm[1] + ':' + fm[1] + ':' + content.length);
        else out.push(nm[1] + '=' + content.toString());
      }
      res.writeHead(200, { 'Content-Type': 'text/plain' });
      res.end(out.join(' ') + '\n');
    });
    return;
  }
  if (req.url === '/ctype') {
    res.writeHead(200, { 'Content-Type': 'text/plain' });
    res.end(ctype.split(';')[0] + '\n');
    return;
  }
  if (req.url === '/headers') {
    const raw = req.rawHeaders, xs = [];
    for (let i = 0; i < raw.length; i += 2) {
      if (raw[i].toLowerCase().startsWith('x-')) xs.push(raw[i] + ': ' + raw[i + 1]);
    }
    xs.sort();
    res.writeHead(200, { 'Content-Type': 'text/plain' });
    res.end(xs.join(' | ') + '\n');
    return;
  }
  res.writeHead(200, { 'Content-Type': 'application/octet-stream' });
  res.end(Buffer.from(Array.from({ length: 256 }, (_, i) => i)));
}).listen(18933, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-srv-18933.js >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-up-server.pid
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18933/ 2>/dev/null && break; sleep 0.25; done
printf 'aaa\n' > /tmp/aux4-curl-up-a.txt
printf 'bbbbb\n' > /tmp/aux4-curl-up-b.txt
```

```afterAll
pkill -f "18933" 2>/dev/null
rm -f /tmp/aux4-curl-up-a.txt /tmp/aux4-curl-up-b.txt /tmp/aux4-curl-out.bin
```

### should upload a file as multipart form data

```execute
aux4 curl request --method POST --upload /tmp/aux4-curl-up-a.txt --uploadField media http://127.0.0.1:18933/upload
```

```expect
media:aux4-curl-up-a.txt:4
```

### should send body entries as extra form fields

```execute
aux4 curl request --method POST --upload /tmp/aux4-curl-up-a.txt --uploadField media --body '{"media_category":"tweet_image"}' http://127.0.0.1:18933/upload
```

```expect
media_category=tweet_image media:aux4-curl-up-a.txt:4
```

### should upload multiple files under the same field

```execute
aux4 curl request --method POST --upload /tmp/aux4-curl-up-a.txt --upload /tmp/aux4-curl-up-b.txt --uploadField files http://127.0.0.1:18933/upload
```

```expect
files:aux4-curl-up-a.txt:4 files:aux4-curl-up-b.txt:6
```

### should name each part with field=path

```execute
aux4 curl request --method POST --upload avatar=/tmp/aux4-curl-up-a.txt --upload banner=/tmp/aux4-curl-up-b.txt http://127.0.0.1:18933/upload
```

```expect
avatar:aux4-curl-up-a.txt:4 banner:aux4-curl-up-b.txt:6
```

### should write a binary response to a file

```execute
aux4 curl request --output /tmp/aux4-curl-out.bin http://127.0.0.1:18933/binary < /dev/null
wc -c < /tmp/aux4-curl-out.bin | tr -d ' '
```

```expect
256
```

### should send every repeated header

```execute
aux4 curl request --header "X-One: 1" --header "X-Two: 2" --header "X-Three: 3" http://127.0.0.1:18933/headers < /dev/null
```

```expect
X-One: 1 | X-Three: 3 | X-Two: 2
```

### should default the multipart subtype to form-data

```execute
aux4 curl request --method POST --upload /tmp/aux4-curl-up-a.txt http://127.0.0.1:18933/ctype
```

```expect
multipart/form-data
```

### should keep a caller-chosen multipart subtype

```execute
aux4 curl request --method POST --upload /tmp/aux4-curl-up-a.txt --header "Content-Type: multipart/related" http://127.0.0.1:18933/ctype
```

```expect
multipart/related
```

### should build a multipart/related body: json metadata part then media part

```execute
aux4 curl request --method POST --header "Content-Type: multipart/related" --body '{"name":"doc","mimeType":"application/vnd.google-apps.document"}' --upload text/markdown=/tmp/aux4-curl-up-a.txt http://127.0.0.1:18933/related
```

```expect
application/json; charset=UTF-8={"name":"doc","mimeType":"application/vnd.google-apps.document"} | text/markdown=aaa
```

### should detect the media part content type from the file extension

```execute
printf '# hi\n' > /tmp/aux4-curl-up.md
aux4 curl request --method POST --header "Content-Type: multipart/related" --body '{"name":"doc"}' --upload /tmp/aux4-curl-up.md http://127.0.0.1:18933/related
rm -f /tmp/aux4-curl-up.md
```

```expect
application/json; charset=UTF-8={"name":"doc"} | text/markdown; charset=utf-8=# hi
```

## with --status

```timeout
20000
```

```beforeAll
cat > /tmp/aux4-curl-srv-18955.js << 'ENDJS'
const http = require('http');
http.createServer((req, res) => {
  if (req.url === '/missing') { res.writeHead(404); res.end('not found\n'); return; }
  if (req.url === '/slow') { setTimeout(() => { res.writeHead(200); res.end('late\n'); }, 3000); return; }
  res.writeHead(200, { 'Content-Type': 'text/plain' });
  res.end('ok\n');
}).listen(18955, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-srv-18955.js >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-status-server.pid
for i in $(seq 1 40); do curl -s -o /dev/null http://127.0.0.1:18955/ 2>/dev/null && break; sleep 0.25; done
```

```afterAll
kill $(cat /tmp/aux4-curl-status-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-status-server.pid
```

### should print only the status code for a 200

```execute
aux4 curl request --status true http://127.0.0.1:18955/ < /dev/null
```

```expect
200
```

### should print only the status code for a 404 and still exit 0

```execute
aux4 curl request --status true http://127.0.0.1:18955/missing < /dev/null
echo "exit=$?"
```

```expect
404
exit=0
```

### should print nothing and exit 1 on a transport failure

```execute
aux4 curl request --status true http://127.0.0.1:1/ < /dev/null
echo "exit=$?"
```

```expect
exit=1
```

```error:partial
Error: *
```

## with --maxTime

```timeout
20000
```

```beforeAll
cat > /tmp/aux4-curl-srv-18956.js << 'ENDJS'
const http = require('http');
http.createServer((req, res) => {
  setTimeout(() => { res.writeHead(200); res.end('late\n'); }, 3000);
}).listen(18956, '127.0.0.1');
ENDJS
nohup node /tmp/aux4-curl-srv-18956.js >/dev/null 2>&1 &
echo $! > /tmp/aux4-curl-maxtime-server.pid
for i in $(seq 1 40); do curl -s -o /dev/null --max-time 1 http://127.0.0.1:18956/ 2>/dev/null; nc -z 127.0.0.1 18956 2>/dev/null && break; sleep 0.25; done
```

```afterAll
kill $(cat /tmp/aux4-curl-maxtime-server.pid) 2>/dev/null
rm -f /tmp/aux4-curl-maxtime-server.pid
```

### should fail with a transport error when the timeout is exceeded

```execute
aux4 curl request --maxTime 1 http://127.0.0.1:18956/slow < /dev/null
echo "exit=$?"
```

```expect
exit=1
```

```error:partial
Error: *Timeout*
```

### should print nothing and exit 1 with --status when the timeout is exceeded

```execute
aux4 curl request --status true --maxTime 1 http://127.0.0.1:18956/slow < /dev/null
echo "exit=$?"
```

```expect
exit=1
```
