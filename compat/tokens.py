import argparse
import os
import sys
from pathlib import Path

def main():
    here = Path(__file__).resolve().parent
    p = argparse.ArgumentParser(description="снапшоты токенов эталона")
    p.add_argument("--ref", default=str(here.parent.parent / "uniland"))
    p.add_argument("--cases", default=str(here / "cases"))
    p.add_argument("--out", default=str(here / "expected-tokens"))
    p.add_argument("--filter", default=None)
    a = p.parse_args()

    sys.path.insert(0, str(Path(a.ref).resolve()))
    from uniland.lexer import Lexer

    cases_dir, out_dir = Path(a.cases).resolve(), Path(a.out).resolve()
    out_dir.mkdir(parents=True, exist_ok=True)

    cases = sorted(cases_dir.glob("*.uni"))
    if a.filter:
        cases = [c for c in cases if a.filter in c.name]
    if not cases:
        sys.exit(f"Кейсы не найдены в {cases_dir}")

    for case in cases:
        src = case.read_text(encoding="utf-8")
        tokens = Lexer(src).tokenize()
        text = "\n".join(repr(t) for t in tokens) + "\n"
        (out_dir / (case.stem + ".txt")).write_text(text, encoding="utf-8", newline="\n")
        print(f"  {case.name}")

    print(f"\nДамп токенов: {len(cases)} кейсов → {out_dir}")

if __name__ == "__main__":
    main()
