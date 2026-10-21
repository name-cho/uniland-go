package std

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/name-cho/uniland-go/internal/interp/val"
	"github.com/name-cho/uniland-go/internal/pyops"
)

func normalizeNL(s string) string {
	b := strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(b, "\r", "\n")
}

func pyWriteNL(s string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(s, "\n", "\r\n")
	}
	return s
}

func bReadFile(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "read_file"); err != nil {
		return nil, err
	}
	path, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("can only join str (not \"%s\") to str", pyops.TypeName(args[0]))
	}
	data, err := os.ReadFile(string(path))
	if err != nil {
		return nil, fmt.Errorf("Failed to read file: %s", fileErrText(err))
	}
	return val.Str(normalizeNL(string(data))), nil
}

func fileErrText(err error) string {
	pe, ok := err.(*os.PathError)
	if !ok {
		return err.Error()
	}
	errno := 2
	reason := pe.Err.Error()
	switch {
	case errors.Is(pe.Err, fs.ErrNotExist):
		reason = "No such file or directory"
	case errors.Is(pe.Err, fs.ErrPermission):
		errno, reason = 13, "Permission denied"
	case errors.Is(pe.Err, fs.ErrExist):
		errno, reason = 17, "File exists"
	}
	if se, ok := pe.Err.(syscall.Errno); ok && errno == 2 && reason == pe.Err.Error() {

		errno = int(se)
	}
	return fmt.Sprintf("[Errno %d] %s: '%s'", errno, reason, pe.Path)
}

func bWriteFile(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "write_file"); err != nil {
		return nil, err
	}
	path, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to write file: str expected, not %s", pyops.TypeName(args[0]))
	}
	content := pyops.Str(args[1])
	if err := os.WriteFile(string(path), []byte(pyWriteNL(content)), 0644); err != nil {
		return nil, fmt.Errorf("Failed to write file: %s", err)
	}
	return val.Nul, nil
}

func bAppendFile(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "append_file"); err != nil {
		return nil, err
	}
	path, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to append to file: str expected, not %s", pyops.TypeName(args[0]))
	}
	f, err := os.OpenFile(string(path), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("Failed to append to file: %s", err)
	}
	if _, err := f.WriteString(pyWriteNL(pyops.Str(args[1]))); err != nil {
		f.Close()
		return nil, fmt.Errorf("Failed to append to file: %s", err)
	}
	f.Close()
	return val.Nul, nil
}

func bFileExists(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "file_exists"); err != nil {
		return nil, err
	}
	path, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("os.stat: path should be string, bytes, os.PathLike or integer, not %s", pyops.TypeName(args[0]))
	}
	_, err := os.Stat(string(path))
	return val.Bool(err == nil || !os.IsNotExist(err)), nil
}

func bDeleteFile(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "delete_file"); err != nil {
		return nil, err
	}
	path, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to delete file: str expected, not %s", pyops.TypeName(args[0]))
	}
	if err := os.Remove(string(path)); err != nil {
		return nil, fmt.Errorf("Failed to delete file: %s", err)
	}
	return val.Nul, nil
}

func bListDir(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "list_dir"); err != nil {
		return nil, err
	}
	path, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to list directory: str expected, not %s", pyops.TypeName(args[0]))
	}
	entries, err := os.ReadDir(string(path))
	if err != nil {
		return nil, fmt.Errorf("Failed to list directory: %s", err)
	}
	items := []val.Value{}
	for _, e := range entries {
		items = append(items, val.Str(e.Name()))
	}
	return val.NewList(items), nil
}

func bCreateDir(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "create_dir"); err != nil {
		return nil, err
	}
	path, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to create directory: str expected, not %s", pyops.TypeName(args[0]))
	}
	if err := os.MkdirAll(string(path), 0755); err != nil {
		return nil, fmt.Errorf("Failed to create directory: %s", err)
	}
	return val.Nul, nil
}

func bCopyFile(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "copy_file"); err != nil {
		return nil, err
	}
	src, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to copy file: str expected, not %s", pyops.TypeName(args[0]))
	}
	dst, ok2 := args[1].(val.Str)
	if !ok2 {
		return nil, fmt.Errorf("Failed to copy file: str expected, not %s", pyops.TypeName(args[1]))
	}
	data, err := os.ReadFile(string(src))
	if err != nil {
		return nil, fmt.Errorf("Failed to copy file: %s", err)
	}
	if err := os.WriteFile(string(dst), data, 0644); err != nil {
		return nil, fmt.Errorf("Failed to copy file: %s", err)
	}
	return val.Nul, nil
}

func bRenameFile(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "rename_file"); err != nil {
		return nil, err
	}
	src, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to rename file: str expected, not %s", pyops.TypeName(args[0]))
	}
	dst, ok2 := args[1].(val.Str)
	if !ok2 {
		return nil, fmt.Errorf("Failed to rename file: str expected, not %s", pyops.TypeName(args[1]))
	}
	if err := os.Rename(string(src), string(dst)); err != nil {
		return nil, fmt.Errorf("Failed to rename file: %s", err)
	}
	return val.Nul, nil
}

func bFileInfo(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "file_info"); err != nil {
		return nil, err
	}
	path, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("file_info(): path does not exist: %s", pyops.PyStr(args[0]))
	}
	st, err := os.Stat(string(path))
	if err != nil {
		return nil, fmt.Errorf("file_info(): path does not exist: %s", string(path))
	}
	out := val.NewMap()
	out.Set("size", val.Int64(st.Size()))
	out.Set("is_dir", val.Bool(st.IsDir()))
	out.Set("is_file", val.Bool(!st.IsDir()))
	mtime := float64(st.ModTime().UnixNano()) / 1e9
	out.Set("modified", val.NewFloat(mtime))
	return out, nil
}

func bOpenFile(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "open_file"); err != nil {
		return nil, err
	}
	target, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to open file: str expected, not %s", pyops.TypeName(args[0]))
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":

		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", string(target))
	case "darwin":
		cmd = exec.Command("open", string(target))
	default:
		cmd = exec.Command("xdg-open", string(target))
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("Failed to open file: %s", err)
	}
	go cmd.Wait()
	return val.Nul, nil
}

func bOpenURL(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "open_url"); err != nil {
		return nil, err
	}
	target, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to open url: str expected, not %s", pyops.TypeName(args[0]))
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", string(target))
	case "darwin":
		cmd = exec.Command("open", string(target))
	default:
		cmd = exec.Command("xdg-open", string(target))
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("Failed to open url: %s", err)
	}
	go cmd.Wait()
	return val.Nul, nil
}

func bZip(args []val.Value) (val.Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("zip() expects (src, out_zip)")
	}
	src, ok := args[0].(val.Str)
	if !ok {
		if l, ok2 := args[0].(*val.List); ok2 {
			paths := []string{}
			for _, x := range l.Items {
				s, ok := x.(val.Str)
				if !ok {
					return nil, fmt.Errorf("zip(): paths must be strings")
				}
				paths = append(paths, string(s))
			}
			return zipPaths(paths, args[1])
		}
		return nil, fmt.Errorf("zip(): paths must be strings")
	}
	return zipPaths([]string{string(src)}, args[1])
}

func zipPaths(paths []string, outV val.Value) (val.Value, error) {
	out, ok := outV.(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to create zip: str expected, not %s", pyops.TypeOf(outV))
	}
	f, err := os.Create(string(out))
	if err != nil {
		return nil, fmt.Errorf("Failed to create zip: %s", err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("Failed to create zip: %s", err)
		}
		if st.IsDir() {
			base := filepath.Base(filepath.Clean(p))
			filepath.Walk(p, func(path string, info os.FileInfo, werr error) error {
				if werr != nil {
					return werr
				}
				if info.IsDir() {
					return nil
				}
				rel, rerr := filepath.Rel(filepath.Dir(filepath.Clean(p)), path)
				if rerr != nil {
					rel = info.Name()
				}
				if err := addToZip(zw, path, rel); err != nil {
					return err
				}
				_ = base
				return nil
			})
		} else {
			if err := addToZip(zw, p, filepath.Base(p)); err != nil {
				return nil, fmt.Errorf("Failed to create zip: %s", err)
			}
		}
	}
	return val.Nul, nil
}

func addToZip(zw *zip.Writer, path, name string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	w, err := zw.Create(filepath.ToSlash(name))
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func bUnzip(args []val.Value) (val.Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("unzip() expects (archive, dest_dir)")
	}
	archive, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to unzip: str expected, not %s", pyops.TypeName(args[0]))
	}
	dest, ok2 := args[1].(val.Str)
	if !ok2 {
		return nil, fmt.Errorf("Failed to unzip: str expected, not %s", pyops.TypeName(args[1]))
	}
	lower := strings.ToLower(string(archive))
	if strings.HasSuffix(lower, ".zip") {
		if err := unzipZip(string(archive), string(dest)); err != nil {
			return nil, fmt.Errorf("Failed to unzip: %s", err)
		}
		return val.Nul, nil
	}
	if strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.bz2") ||
		strings.HasSuffix(lower, ".tbz2") || strings.HasSuffix(lower, ".tar.xz") ||
		strings.HasSuffix(lower, ".txz") {
		if err := untar(string(archive), string(dest)); err != nil {
			return nil, fmt.Errorf("Failed to unzip: %s", err)
		}
		return val.Nul, nil
	}
	return nil, fmt.Errorf("Failed to unzip: unknown archive format '%s'", filepath.Ext(string(archive)))
}

func unzipZip(archive, dest string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	os.MkdirAll(dest, 0755)
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(filepath.Join(dest, filepath.FromSlash(f.Name)))
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func untar(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	tr := (*tar.Reader)(nil)
	lower := strings.ToLower(archive)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		tr = tar.NewReader(gz)
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"),
		strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		return fmt.Errorf("unsupported compression for unpack_archive: bz2/xz")
	default:
		tr = tar.NewReader(f)
	}
	os.MkdirAll(dest, 0755)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeDir {
			os.MkdirAll(filepath.Join(dest, hdr.Name), 0755)
			continue
		}
		out, err := os.Create(filepath.Join(dest, hdr.Name))
		if err != nil {
			return err
		}
		_, err = io.Copy(out, tr)
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func bZipList(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "zip_list"); err != nil {
		return nil, err
	}
	archive, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to read zip: str expected, not %s", pyops.TypeName(args[0]))
	}
	r, err := zip.OpenReader(string(archive))
	if err != nil {
		return nil, fmt.Errorf("Failed to read zip: %s", err)
	}
	defer r.Close()
	items := []val.Value{}
	for _, f := range r.File {
		items = append(items, val.Str(f.Name))
	}
	return val.NewList(items), nil
}

func bReadBase64(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "read_base64"); err != nil {
		return nil, err
	}
	path, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to read file: str expected, not %s", pyops.TypeName(args[0]))
	}
	data, err := os.ReadFile(string(path))
	if err != nil {
		return nil, fmt.Errorf("Failed to read file: %s", fileErrText(err))
	}
	return val.Str(base64.StdEncoding.EncodeToString(data)), nil
}

func bWriteBase64(args []val.Value) (val.Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("write_base64() expects (path, base64_string)")
	}
	path, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to write file: str expected, not %s", pyops.TypeName(args[0]))
	}
	b64, ok2 := args[1].(val.Str)
	if !ok2 {
		return nil, fmt.Errorf("Failed to write file: a bytes-like object is required, not '%s'", pyops.TypeName(args[1]))
	}
	data, err := base64.StdEncoding.DecodeString(string(b64))
	if err != nil {
		return nil, fmt.Errorf("Failed to write file: %s", err)
	}
	if err := os.WriteFile(string(path), data, 0644); err != nil {
		return nil, fmt.Errorf("Failed to write file: %s", err)
	}
	return val.Nul, nil
}

func bBase64Encode(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "base64_encode"); err != nil {
		return nil, err
	}
	return val.Str(base64.StdEncoding.EncodeToString([]byte(pyops.PyStr(args[0])))), nil
}

func bBase64Decode(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "base64_decode"); err != nil {
		return nil, err
	}
	b64, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("base64_decode failed: argument should be a bytes-like object or ASCII string, not '%s'", pyops.TypeName(args[0]))
	}

	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r == '+', r == '/', r == '=':
			return r
		}
		return -1
	}, string(b64))
	data, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("base64_decode failed: %s", err)
	}
	return val.Str(string(data)), nil
}

func bGetTime(args []val.Value) (val.Value, error) {
	return val.NewFloat(float64(time.Now().UnixNano()) / 1e9), nil
}

func bGetDate(args []val.Value) (val.Value, error) {
	t := time.Now()
	micros := t.Nanosecond() / 1000
	base := t.Format("2006-01-02T15:04:05")
	if micros != 0 {
		return val.Str(fmt.Sprintf("%s.%06d", base, micros)), nil
	}
	return val.Str(base), nil
}

func bSleep(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "sleep"); err != nil {
		return nil, err
	}
	f, ok, _ := asFloatOf(args[0])
	if !ok {
		return nil, fmt.Errorf("a float is required, not %s", pyops.TypeName(args[0]))
	}
	if f < 0 {
		return nil, fmt.Errorf("sleep length must be non-negative")
	}
	time.Sleep(time.Duration(f * float64(time.Millisecond)))
	return val.Nul, nil
}

func bFormatTime(args []val.Value) (val.Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("format_time() expects (timestamp[, format])")
	}
	ts, ok, _ := asFloatOf(args[0])
	if !ok {
		return nil, fmt.Errorf("an integer is required, not %s", pyops.TypeName(args[0]))
	}
	format := "%Y-%m-%d %H:%M:%S"
	if len(args) == 2 {
		fm, ok := args[1].(val.Str)
		if !ok {
			return nil, fmt.Errorf("strftime() argument must be str, not %s", pyops.TypeName(args[1]))
		}
		format = string(fm)
	}
	sec := int64(ts)
	frac := ts - float64(sec)
	t := time.Unix(sec, int64(frac*1e9))
	return val.Str(strftime(t, format)), nil
}

func strftime(t time.Time, format string) string {
	pad2 := func(x int) string { return fmt.Sprintf("%02d", x) }
	var b strings.Builder
	runes := []rune(format)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '%' {
			b.WriteRune(runes[i])
			continue
		}
		if i+1 >= len(runes) {
			b.WriteByte('%')
			break
		}
		i++
		switch runes[i] {
		case 'Y':
			b.WriteString(t.Format("2006"))
		case 'y':
			b.WriteString(pad2(t.Year() % 100))
		case 'm':
			b.WriteString(pad2(int(t.Month())))
		case 'd':
			b.WriteString(pad2(t.Day()))
		case 'H':
			b.WriteString(pad2(t.Hour()))
		case 'M':
			b.WriteString(pad2(t.Minute()))
		case 'S':
			b.WriteString(pad2(t.Second()))
		case 'f':
			b.WriteString(fmt.Sprintf("%06d", t.Nanosecond()/1000))
		case 'j':
			b.WriteString(fmt.Sprintf("%03d", t.YearDay()))
		case 'b':
			b.WriteString(t.Format("Jan"))
		case 'B':
			b.WriteString(t.Format("January"))
		case 'a':
			b.WriteString(t.Format("Mon"))
		case 'A':
			b.WriteString(t.Format("Monday"))
		case 'p':
			if t.Hour() < 12 {
				b.WriteString("AM")
			} else {
				b.WriteString("PM")
			}
		case 'I':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			b.WriteString(pad2(h))
		case 'Z':
			b.WriteString(t.Format("MST"))
		case '%':
			b.WriteByte('%')
		default:
			b.WriteByte('%')
			b.WriteRune(runes[i])
		}
	}
	return b.String()
}

func bParseJSON(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "parse_json"); err != nil {
		return nil, err
	}
	s, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("the JSON object must be str, bytes or bytearray, not %s", pyops.TypeName(args[0]))
	}
	v, err := jsonParse(string(s))
	if err != nil {
		return nil, fmt.Errorf("JSON parse error: %s", err)
	}
	return v, nil
}

func bStringifyJSON(args []val.Value) (val.Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("stringify_json() expects (value[, indent])")
	}
	var indent *int
	if len(args) == 2 {
		n, err := pyops.IntOf(args[1])
		if err != nil {
			return nil, fmt.Errorf("JSON stringify error: %s", err)
		}
		iv := n.Int64()
		if iv > 0 && iv < math.MaxInt32 {
			p := int(iv)
			indent = &p
		}
	}
	out, err := jsonDumps(args[0], indent)
	if err != nil {
		return nil, fmt.Errorf("JSON stringify error: %s", err)
	}
	return val.Str(out), nil
}

func bHTTP(method string) Native {
	return func(args []val.Value) (val.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("http_%s() needs a url string", method)
		}
		url, ok := args[0].(val.Str)
		if !ok {
			return nil, fmt.Errorf("http_%s() needs a url string", method)
		}
		var body io.Reader
		var headers http.Header
		if (method == "post" || method == "put") && len(args) > 1 {
			data := args[1]
			payload, err := jsonDumps(data, nil)
			if err != nil {
				return nil, fmt.Errorf("HTTP %s failed: %s", strings.ToUpper(method), err)
			}
			body = strings.NewReader(payload)
			headers = http.Header{}
			headers.Set("content-type", "application/json")
		}
		req, err := http.NewRequest(strings.ToUpper(method), string(url), body)
		if err != nil {
			return nil, fmt.Errorf("HTTP %s failed: %s", strings.ToUpper(method), err)
		}
		for k, v := range headers {
			req.Header.Set(k, v[0])
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP %s failed: %s", strings.ToUpper(method), shortenHTTPErr(err))
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("HTTP %s failed: %s", strings.ToUpper(method), err)
		}
		out := val.NewMap()
		out.Set("status", val.Int64(int64(resp.StatusCode)))
		out.Set("body", val.Str(normalizeNL(string(raw))))
		h := val.NewMap()
		keys := make([]string, 0, len(resp.Header))
		for k := range resp.Header {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			h.Set(k, val.Str(strings.Join(resp.Header[k], ", ")))
		}
		out.Set("headers", h)
		return out, nil
	}
}

func shortenHTTPErr(err error) string {
	s := err.Error()

	return s
}

func bSystem(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "system"); err != nil {
		return nil, err
	}
	cmdStr, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("Failed to execute command: str expected, not %s", pyops.TypeName(args[0]))
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		cmd = exec.Command(comspec, "/c", string(cmdStr))
	} else {
		cmd = exec.Command("/bin/sh", "-c", string(cmdStr))
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil && cmd.ProcessState == nil {
		return nil, fmt.Errorf("Failed to execute command: %s", err)
	}
	o := val.NewMap()
	o.Set("stdout", val.Str(normalizeNL(outBuf.String())))
	o.Set("stderr", val.Str(normalizeNL(errBuf.String())))
	rc := 0
	if cmd.ProcessState != nil {
		rc = cmd.ProcessState.ExitCode()
	}
	o.Set("returncode", val.Int64(int64(rc)))
	return o, nil
}

func bEnv(args []val.Value) (val.Value, error) {
	if len(args) == 0 {
		out := val.NewMap()
		for _, e := range os.Environ() {
			kv := strings.SplitN(e, "=", 2)
			if len(kv) == 2 {
				out.Set(kv[0], val.Str(kv[1]))
			}
		}
		return out, nil
	}
	name, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("key must be str, not %s", pyops.TypeName(args[0]))
	}
	if v, has := os.LookupEnv(string(name)); has {
		return val.Str(v), nil
	}
	if len(args) > 1 {
		return args[1], nil
	}
	return val.Nul, nil
}

func bInput(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		prompt := ""
		if len(args) > 0 {
			prompt = pyops.Str(args[0])
		}
		fmt.Fprint(ctx.Out, prompt)
		if ctx.In == nil {
			return val.Str(""), nil
		}

		if ctx.RdIn == nil {
			ctx.RdIn = bufio.NewReader(ctx.In)
		}
		line, err := ctx.RdIn.ReadString('\n')
		if err != nil && line == "" {

			return val.Str(""), nil
		}

		return val.Str(strings.TrimSuffix(normalizeNL(line), "\n")), nil
	}
}

func bExit(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {

		code := big.NewInt(0)
		if len(args) > 0 {
			c, err := pyops.IntOf(args[0])
			if err != nil {
				return nil, err
			}
			code = c
		}
		i := 0
		if !code.IsInt64() {
			if code.Sign() > 0 {
				i = math.MaxInt32
			} else {
				i = math.MinInt32
			}
		} else {
			i = int(code.Int64())
		}

		return nil, ctx.OnExit(i)
	}
}

func bArgs(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		items := make([]val.Value, len(ctx.ScriptArgs))
		for i, a := range ctx.ScriptArgs {
			items[i] = val.Str(a)
		}
		return val.NewList(items), nil
	}
}

func All(ctx *Ctx) map[string]Native {
	rnd := newRandomSource()
	return map[string]Native{

		"print": bPrint(ctx), "output": bPrint(ctx), "input": bInput(ctx),
		"log_info": logger(ctx, "[INFO]"), "log_debug": logger(ctx, "[DEBUG]"),
		"log_warn": logger(ctx, "[WARN]"), "log_error": logger(ctx, "[ERROR]"),
		"log_fatal": logger(ctx, "[FATAL]"),

		"typeof": bTypeof, "to_string": bToString, "to_number": bToNumber,
		"to_boolean": bToBoolean,
		"is_number":  isCheck("is_number", isNumber),
		"is_string":  isCheck("is_string", func(v val.Value) bool { _, ok := v.(val.Str); return ok }),
		"is_array":   isCheck("is_array", func(v val.Value) bool { _, ok := v.(*val.List); return ok }),
		"is_object":  isCheck("is_object", func(v val.Value) bool { _, ok := v.(*val.Map); return ok }),
		"is_boolean": isCheck("is_boolean", func(v val.Value) bool {
			_, ok := v.(val.Bool)
			return ok
		}),
		"is_null": isCheck("is_null", func(v val.Value) bool { return v == nil || isNullValue(v) }),
		"is_function": isCheck("is_function", func(v val.Value) bool {
			return !isNumber(v) && !isString(v) && !isList(v) && !isMapVal(v) && !isBoolVal(v) && !isNullValue(v)
		}),

		"length": bLength, "split": bSplit, "join": bJoin, "replace": bReplace,
		"substring": bSubstring, "trim": bTrim, "uppercase": bUpper,
		"lowercase": bLower, "startswith": bStartswith, "endswith": bEndswith,
		"repeat": bRepeat, "char_code": bCharCode, "from_char_code": bFromCharCode,

		"push": bPush, "pop": bPop, "shift": bShift, "unshift": bUnshift,
		"slice": bSlice, "splice": bSplice, "reverse": bReverse, "sort": bSort(ctx),
		"includes": bIncludes, "indexOf": bIndexOf, "map": bMap(ctx), "filter": bFilter(ctx),
		"reduce": bReduce(ctx), "find": bFind(ctx), "each": bEach(ctx), "sum": bSum, "range": bRange,

		"keys": bKeys, "values": bValues, "entries": bEntries, "has": bHas,
		"merge": bMerge, "clone": bClone,

		"abs": bAbs, "round": bRound, "floor": bFloor, "ceil": bCeil,
		"sqrt": bSqrt, "min": bMin, "max": bMax, "pow": bPow,
		"random": bRandom(rnd), "random_int": bRandomInt(rnd),

		"regex_match": bRegexMatch, "regex_findall": bRegexFindall,
		"regex_replace": bRegexReplace,

		"read_file": bReadFile, "write_file": bWriteFile, "append_file": bAppendFile,
		"file_exists": bFileExists, "delete_file": bDeleteFile, "list_dir": bListDir,
		"create_dir": bCreateDir, "copy_file": bCopyFile, "rename_file": bRenameFile,
		"file_info": bFileInfo,

		"open_file": bOpenFile, "open_url": bOpenURL,
		"zip": bZip, "unzip": bUnzip, "zip_list": bZipList,
		"read_base64": bReadBase64, "write_base64": bWriteBase64,
		"base64_encode": bBase64Encode, "base64_decode": bBase64Decode,

		"get_time": bGetTime, "get_date": bGetDate, "sleep": bSleep,
		"format_time": bFormatTime,

		"parse_json": bParseJSON, "stringify_json": bStringifyJSON,

		"http_get": bHTTP("get"), "http_post": bHTTP("post"),
		"http_put": bHTTP("put"), "http_delete": bHTTP("delete"),

		"system": bSystem, "env": bEnv, "exit": bExit(ctx), "argv": bArgs(ctx),

		"gui_window": bGUIWindow(ctx), "gui_alert": bGUIAlert(ctx),
		"gui_ask": bGUIAsk(ctx), "gui_confirm": bGUIConfirm(ctx),
	}
}

func isNullValue(v val.Value) bool {
	_, ok := v.(val.Null)
	return ok
}
func isString(v val.Value) bool  { _, ok := v.(val.Str); return ok }
func isBoolVal(v val.Value) bool { _, ok := v.(val.Bool); return ok }
func isList(v val.Value) bool    { _, ok := v.(*val.List); return ok }
func isMapVal(v val.Value) bool  { _, ok := v.(*val.Map); return ok }
