import io, json, sys, contextlib

def use_utf8(stream, errors):
    try:
        stream.reconfigure(encoding="utf-8", errors=errors)
    except Exception:
        pass

use_utf8(sys.stdin, "replace")
use_utf8(sys.stdout, "replace")
use_utf8(sys.stderr, "replace")

py_ns = {"__name__": "uniland_embedded"}
_wid = [1000]

_RAW = sys.stdout

def send(obj):
    _RAW.write(json.dumps(obj, ensure_ascii=True) + "\n")
    _RAW.flush()

def rpc(op, **kw):
    _wid[0] += 1
    rid = _wid[0]
    msg = {"kind": "req", "id": rid, "op": op}
    msg.update(kw)
    send(msg)
    while True:
        line = sys.stdin.readline()
        if not line:
            raise RuntimeError("host closed")
        resp = json.loads(line)
        if resp.get("id") == rid:
            if resp.get("ok"):
                return resp.get("value")
            raise RuntimeError(resp.get("err", "bridge error"))

class UniBridge:

    def __getattr__(self, key):
        r = rpc("uni_get", k=key)
        return dec(r)

    def __setattr__(self, key, value):
        try:
            del self.__dict__[key]
        except KeyError:
            pass
        object.__setattr__(self, "_setkey", key)
        rpc("uni_set", k=key, v=value)
        object.__delattr__(self, "_setkey")

    def __getitem__(self, key):
        r = rpc("uni_get", k=key)
        return dec(r)

    def __setitem__(self, key, value):
        rpc("uni_set", k=key, v=value)

    def call(self, name, *args):
        r = rpc("uni_call", k=name, args=[enc(a) for a in args])
        return dec(r)

uni_bridge = UniBridge()
py_ns["uni"] = uni_bridge

def enc(v):
    if v is None:
        return None
    if v is True:
        return True
    if v is False:
        return False
    if isinstance(v, int):
        return {"T": "int", "v": str(v)}
    if isinstance(v, float):
        f = repr(v)
        if f in ("inf", "-inf", "nan"):
            return {"T": "floats", "v": f}
        return {"T": "float", "v": repr(v)}
    if isinstance(v, str):
        return {"T": "str", "v": v}
    if isinstance(v, (list, tuple)):
        return {"T": "list", "v": [enc(x) for x in v]}
    if isinstance(v, dict):
        return {"T": "obj", "k": [str(k) for k in v], "v": {str(k): enc(x) for k, x in v.items()}}
    return {"T": "func"}

def dec(w):
    if w is None or isinstance(w, (bool, int, float)):
        return w
    if isinstance(w, str):
        return w
    if isinstance(w, list):
        return [dec(x) for x in w]
    if isinstance(w, dict):
        t = w.get("T")
        if t == "int":
            return int(w["v"])
        if t == "float":
            return float(w["v"])
        if t == "str":
            return w["v"]
        if t == "list":
            return [dec(x) for x in w["v"]]
        if t == "obj":
            src = w.get("v", {})
            keys = w.get("k")
            if isinstance(keys, list):
                return {k: dec(src[k]) for k in keys}
            return {k: dec(x) for k, x in src.items()}
        if t == "null":
            return None
        if t == "func":
            return None
    return None

def emit_out(text):
    if text:
        send({"kind": "out", "text": text})

def emit_err(text):
    if text:
        send({"kind": "err", "text": text})

def run_exec(code):
    buf_out, buf_err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(buf_out), contextlib.redirect_stderr(buf_err):
        try:
            exec(code, py_ns)
            ok = True
            err = None
        except Exception as e:
            ok = False
            err = str(e)
    emit_out(buf_out.getvalue())
    emit_err(buf_err.getvalue())
    return ok, err

def run_eval(code):
    buf_out, buf_err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(buf_out), contextlib.redirect_stderr(buf_err):
        try:
            value = eval(code, py_ns)
            ok = True
            err = None
        except Exception as e:
            ok = False
            err = str(e)
            value = None
    emit_out(buf_out.getvalue())
    emit_err(buf_err.getvalue())
    return ok, err, value

def main_loop():
    while True:
        line = sys.stdin.readline()
        if not line:
            break
        try:
            msg = json.loads(line)
        except Exception:
            continue
        op = msg.get("op")
        rid = msg.get("id")
        if op == "shutdown":
            break
        if op == "exec":
            ok, err = run_exec(msg["code"])
            send({"id": rid, "ok": ok, "err": err})
        elif op == "eval":
            ok, err, value = run_eval(msg["code"])
            send({"id": rid, "ok": ok, "err": err, "value": enc(value)})
        else:
            send({"id": rid, "ok": False, "err": "unknown op"})

if __name__ == "__main__":
    main_loop()
