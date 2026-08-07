package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func init() {
	// Go's mime package only knows a handful of types and otherwise relies on
	// the OS mime.types database, which a minimal Linux runner may not have.
	// Register the source formats Drive can convert into a Doc so uploads get
	// the right Content-Type everywhere (Markdown especially — without this a
	// .md file is treated as plain text and its formatting is lost).
	_ = mime.AddExtensionType(".md", "text/markdown")
	_ = mime.AddExtensionType(".markdown", "text/markdown")
	_ = mime.AddExtensionType(".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	_ = mime.AddExtensionType(".odt", "application/vnd.oasis.opendocument.text")
	_ = mime.AddExtensionType(".rtf", "application/rtf")
	_ = mime.AddExtensionType(".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	_ = mime.AddExtensionType(".ods", "application/vnd.oasis.opendocument.spreadsheet")
	_ = mime.AddExtensionType(".csv", "text/csv")
	_ = mime.AddExtensionType(".tsv", "text/tab-separated-values")
}

// parseMultiValue reads a repeatable variable. aux4 passes `values(name*)` as a
// JSON array; a plain string (or newline-separated list) is accepted too so a
// single value keeps working.
func parseMultiValue(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	if strings.HasPrefix(raw, "[") {
		var values []string
		if err := json.Unmarshal([]byte(raw), &values); err == nil {
			result := make([]string, 0, len(values))
			for _, v := range values {
				if strings.TrimSpace(v) != "" {
					result = append(result, v)
				}
			}
			return result
		}
	}

	result := make([]string, 0)
	for _, v := range strings.Split(raw, "\n") {
		if strings.TrimSpace(v) != "" {
			result = append(result, v)
		}
	}
	return result
}

// addFilePart writes one file to the multipart body under the given form field.
func addFilePart(writer *multipart.Writer, uploadField string, uploadPath string) error {
	file, err := os.Open(uploadPath)
	if err != nil {
		return fmt.Errorf("could not open file to upload: %v", err)
	}
	defer file.Close()

	contentType := mime.TypeByExtension(filepath.Ext(uploadPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	header := make(map[string][]string)
	header["Content-Disposition"] = []string{
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, uploadField, filepath.Base(uploadPath)),
	}
	header["Content-Type"] = []string{contentType}

	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	return nil
}

// buildMultipartBody builds a multipart body from one or more upload entries.
// Each entry is either a bare path (sent under uploadField) or "field=path" to
// name that part explicitly. When body is a non-empty JSON object its entries
// are added as additional text fields. It returns the generated boundary so the
// caller can decide which multipart subtype to declare.
func buildMultipartBody(uploads []string, uploadField string, body string) (io.Reader, string, error) {
	if uploadField == "" {
		uploadField = "file"
	}

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	if body != "" {
		var fields map[string]interface{}
		if err := json.Unmarshal([]byte(body), &fields); err != nil {
			return nil, "", fmt.Errorf("body must be a JSON object when uploading a file: %v", err)
		}
		for name, value := range fields {
			if err := writer.WriteField(name, fmt.Sprintf("%v", value)); err != nil {
				return nil, "", err
			}
		}
	}

	for _, upload := range uploads {
		upload = strings.TrimSpace(upload)
		if upload == "" {
			continue
		}

		field := uploadField
		path := upload
		// "field=path" names the part explicitly. A bare path that itself
		// contains '=' must be given as "field=path" to be unambiguous.
		if index := strings.Index(upload, "="); index > 0 {
			candidate := upload[:index]
			if !strings.ContainsAny(candidate, "/\\.") {
				field = candidate
				path = upload[index+1:]
			}
		}

		if err := addFilePart(writer, field, path); err != nil {
			return nil, "", err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return &buffer, writer.Boundary(), nil
}

// multipartContentTypeFor decides the Content-Type for a multipart request. The
// boundary is generated here so it always has to be ours, but the caller may
// choose the subtype (multipart/related, multipart/mixed, ...) by passing a
// multipart Content-Type header; anything else falls back to form-data.
func multipartContentTypeFor(headers []string, boundary string) string {
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Type") {
			continue
		}

		value := strings.TrimSpace(parts[1])
		if !strings.HasPrefix(strings.ToLower(value), "multipart/") {
			continue
		}

		// Drop any boundary the caller supplied; only ours is valid.
		kept := make([]string, 0)
		for _, attr := range strings.Split(value, ";") {
			attr = strings.TrimSpace(attr)
			if attr == "" || strings.HasPrefix(strings.ToLower(attr), "boundary=") {
				continue
			}
			kept = append(kept, attr)
		}
		return strings.Join(kept, "; ") + "; boundary=" + boundary
	}

	return "multipart/form-data; boundary=" + boundary
}

// isMultipartRelated reports whether the caller asked for a multipart/related
// body via the Content-Type header (as opposed to the default form-data).
func isMultipartRelated(headers []string) bool {
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Type") {
			continue
		}
		return strings.HasPrefix(strings.ToLower(strings.TrimSpace(parts[1])), "multipart/related")
	}
	return false
}

// buildRelatedBody builds a multipart/related body: an optional JSON metadata
// part followed by one media part per upload. This is the shape Google APIs
// expect for uploadType=multipart (Drive convert-on-import, Gmail media, ...).
// The body is the metadata part verbatim (as application/json), NOT split into
// form fields the way form-data uploads are. Each upload is a bare path (media
// Content-Type detected from the extension) or "mime/type=path" to set it
// explicitly.
func buildRelatedBody(uploads []string, metadata string) (io.Reader, string, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	if strings.TrimSpace(metadata) != "" {
		var probe interface{}
		if err := json.Unmarshal([]byte(metadata), &probe); err != nil {
			return nil, "", fmt.Errorf("body must be JSON for a multipart/related request: %v", err)
		}
		header := textproto.MIMEHeader{}
		header.Set("Content-Type", "application/json; charset=UTF-8")
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, "", err
		}
		if _, err := io.WriteString(part, metadata); err != nil {
			return nil, "", err
		}
	}

	for _, upload := range uploads {
		upload = strings.TrimSpace(upload)
		if upload == "" {
			continue
		}

		contentType := ""
		path := upload
		// "mime/type=path" sets the media part's Content-Type explicitly; a bare
		// path falls back to detection from the file extension.
		if index := strings.Index(upload, "="); index > 0 {
			candidate := upload[:index]
			if strings.Contains(candidate, "/") {
				contentType = candidate
				path = upload[index+1:]
			}
		}
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(path))
			if contentType == "" {
				contentType = "application/octet-stream"
			}
		}

		file, err := os.Open(path)
		if err != nil {
			return nil, "", fmt.Errorf("could not open file to upload: %v", err)
		}
		header := textproto.MIMEHeader{}
		header.Set("Content-Type", contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			file.Close()
			return nil, "", err
		}
		if _, err := io.Copy(part, file); err != nil {
			file.Close()
			return nil, "", err
		}
		file.Close()
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return &buffer, writer.Boundary(), nil
}

// args: method url header body showHeaders upload uploadField output bodyFile status maxTime
func runRequest(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: URL is required\n")
		os.Exit(1)
	}

	method := strings.ToUpper(args[0])
	if method == "" {
		method = "GET"
	}

	url := args[1]
	if url == "" {
		fmt.Fprintf(os.Stderr, "Error: URL is required\n")
		os.Exit(1)
	}

	var headers []string
	if len(args) > 2 {
		headers = parseMultiValue(args[2])
	}

	body := ""
	if len(args) > 3 {
		body = args[3]
	}

	showHeaders := false
	if len(args) > 4 && args[4] == "true" {
		showHeaders = true
	}

	var uploads []string
	if len(args) > 5 {
		uploads = parseMultiValue(args[5])
	}

	uploadField := ""
	if len(args) > 6 {
		uploadField = args[6]
	}

	output := ""
	if len(args) > 7 {
		output = args[7]
	}

	bodyFile := ""
	if len(args) > 8 {
		bodyFile = args[8]
	}

	// --status: print only the numeric HTTP status code and nothing else.
	statusOnly := false
	if len(args) > 9 && args[9] == "true" {
		statusOnly = true
	}

	// --maxTime: request timeout in seconds (decimals allowed). 0 = no timeout.
	maxTime := 0.0
	if len(args) > 10 && args[10] != "" {
		if v, err := strconv.ParseFloat(args[10], 64); err == nil && v > 0 {
			maxTime = v
		}
	}

	var reqBody io.Reader
	multipartBoundary := ""

	bodyFileSize := int64(-1)

	if bodyFile != "" {
		// A raw file as the whole request body (as opposed to --upload, which
		// wraps it in a multipart form).
		file, err := os.Open(bodyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening body file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		// Without an explicit length Go streams an *os.File with chunked
		// transfer-encoding, which many APIs reject for uploads.
		if info, err := file.Stat(); err == nil {
			bodyFileSize = info.Size()
		}

		if method == "GET" {
			method = "POST"
		}
		reqBody = file
	} else if len(uploads) > 0 {
		// A file upload is never a GET; default to POST unless told otherwise.
		if method == "GET" {
			method = "POST"
		}
		var err error
		if isMultipartRelated(headers) {
			// Drive/Gmail style: JSON metadata part + media part(s).
			reqBody, multipartBoundary, err = buildRelatedBody(uploads, body)
		} else {
			reqBody, multipartBoundary, err = buildMultipartBody(uploads, uploadField, body)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err == nil && len(data) > 0 {
				reqBody = bytes.NewReader(data)
			}
		}
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		os.Exit(1)
	}

	if bodyFileSize >= 0 {
		req.ContentLength = bodyFileSize
	}

	for _, h := range headers {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	// The boundary is generated here, so the Content-Type has to be rebuilt --
	// but the caller may still choose the multipart subtype.
	if multipartBoundary != "" {
		req.Header.Set("Content-Type", multipartContentTypeFor(headers, multipartBoundary))
	}

	client := &http.Client{}
	if maxTime > 0 {
		client.Timeout = time.Duration(maxTime * float64(time.Second))
	}
	resp, err := client.Do(req)
	if err != nil {
		// Transport failure (DNS/refused/TLS/timeout). With --status, keep stdout
		// empty and exit non-zero so callers can distinguish "unreachable" from a
		// real HTTP status code.
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// --status short-circuits everything else: emit just the code and exit 0
	// regardless of status class, so 200/404/503 all succeed for health checks.
	if statusOnly {
		fmt.Fprintf(os.Stdout, "%d\n", resp.StatusCode)
		return
	}

	if showHeaders {
		fmt.Fprintf(os.Stdout, "HTTP/%d.%d %s\n", resp.ProtoMajor, resp.ProtoMinor, resp.Status)
		for key, values := range resp.Header {
			for _, v := range values {
				fmt.Fprintf(os.Stdout, "%s: %s\n", key, v)
			}
		}
		fmt.Fprintln(os.Stdout)
	}

	if output != "" {
		// Never save an error response as if it were the requested file.
		if resp.StatusCode >= 400 {
			fmt.Fprintf(os.Stderr, "Error: HTTP %d\n", resp.StatusCode)
			io.Copy(os.Stderr, resp.Body)
			fmt.Fprintln(os.Stderr)
			os.Exit(1)
		}

		file, err := os.Create(output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		if _, err := io.Copy(file, resp.Body); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading response: %v\n", err)
		os.Exit(1)
	}
}
