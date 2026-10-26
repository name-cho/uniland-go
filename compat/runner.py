import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

CASE_TIMEOUT = 30
MANUAL = {"t11_golang.uni", "t12_gui.uni"}

def parse_args():
    here = Path(__file__).resolve().parent
    p = argparse.ArgumentParser(description="Снапшот эталонного uniland для golden-тестов")
    p.add_argument("--ref", default=str(here.parent.parent / "uniland"),
                   help="путь к клону питоновского uniland")
    p.add_argument("--cases", default=str(here / "cases"))
    p.add_argument("--out", default=str(here / "expected"))
    p.add_argument("--python", default=sys.executable, help="питон для эталона")
    p.add_argument("--filter", default=None, help="подстрока имени кейса")
    return p.parse_args()

def read_sidecar(case: Path, suffix: str) -> str:
    p = case.with_suffix(suffix)
    return p.read_text(encoding="utf-8") if p.exists() else ""

def parse_args_sidecar(text: str):
    return [ln for ln in text.splitlines() if ln != ""]

def main():
    try:
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
        sys.stderr.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass

    a = parse_args()
    ref, cases_dir, out_dir = Path(a.ref).resolve(), Path(a.cases).resolve(), Path(a.out).resolve()

    if not (ref / "uniland").is_dir():
        sys.exit(f"Эталон не найден: {ref} (клонируй питоновский uniland рядом)")

    out_dir.mkdir(parents=True, exist_ok=True)

    cases = sorted(cases_dir.glob("*.uni"))
    if a.filter:
        cases = [c for c in cases if a.filter in c.name]
    if not cases:
        sys.exit(f"Кейсы не найдены в {cases_dir}")

    for case in [c for c in cases if c.name in MANUAL]:
        print(f"  {case.name:<28} снимок написан вручную, пропуск")
    cases = [c for c in cases if c.name not in MANUAL]
    if not cases:
        print("Нечего снимать: все выбранные кейсы со снимками вручную")
        return

    env = os.environ.copy()
    env["PYTHONPATH"] = str(ref) + (os.pathsep + env["PYTHONPATH"] if env.get("PYTHONPATH") else "")
    env["PYTHONIOENCODING"] = "utf-8"
    env["PYTHONUTF8"] = "1"

    results = []
    for case in cases:
        args = parse_args_sidecar(read_sidecar(case, ".args"))
        stdin = read_sidecar(case, ".stdin")

        cmd = [a.python, "-m", "uniland", "run", case.name, *args]
        try:
            proc = subprocess.run(
                cmd, cwd=str(cases_dir), env=env,
                input=stdin.encode("utf-8"),
                stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                timeout=CASE_TIMEOUT,
            )
            stdout = proc.stdout.decode("utf-8", "replace")
            stderr = proc.stderr.decode("utf-8", "replace")
            exit_code = proc.returncode
        except subprocess.TimeoutExpired:
            stdout, stderr, exit_code = "", f"timeout {CASE_TIMEOUT}s", -99

        record = {
            "case": case.name,
            "args": args,
            "stdin": stdin,
            "stdout": stdout,
            "stderr": stderr,
            "exit": exit_code,
        }
        (out_dir / (case.stem + ".json")).write_text(
            json.dumps(record, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
        mark = "ok" if exit_code == 0 else f"exit {exit_code}"
        results.append((case.name, mark))
        print(f"  {case.name:<28} {mark}")

    print(f"\nСнапшотов: {len(results)} → {out_dir}")

if __name__ == "__main__":
    main()
