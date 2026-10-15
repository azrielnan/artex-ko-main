#!/usr/bin/env python3
"""检查用户可见资源是否意外混入韩文。

范围刻意避开 mock 固定数据、注释和协议原文；mock 演示数据由独立 demo 资源维护，
而本门禁保护真实应用界面和静态导出结果。脚本只依赖 Python 标准库。
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

HANGUL = re.compile(r"[\uac00-\ud7a3\u1100-\u11ff\u3130-\u318f]")
STRING = re.compile(r"(?:\"(?:\\.|[^\"\\])*\"|'(?:\\.|[^'\\])*'|`(?:\\.|[^`\\])*`)")
COMMENT = re.compile(r"//.*$|/\*.*?\*/", re.MULTILINE | re.DOTALL)


def source_root() -> Path:
    return Path(__file__).resolve().parents[1]


def check_source(root: Path) -> list[tuple[Path, int, str]]:
    web_src = root / "web" / "src"
    candidates = [
        *web_src.glob("config/**/*.ts"),
        *web_src.glob("app/**/*.ts"),
        *web_src.glob("app/**/*.tsx"),
        *web_src.glob("components/**/*.ts"),
        *web_src.glob("components/**/*.tsx"),
        *web_src.glob("lib/*.ts"),
    ]
    offenders: list[tuple[Path, int, str]] = []
    for path in sorted(set(candidates)):
        if "mock" in path.parts:
            continue
        text = path.read_text(encoding="utf-8", errors="ignore")
        without_comments = COMMENT.sub("", text)
        for line_no, line in enumerate(without_comments.splitlines(), 1):
            # 后端错误码/兼容哨兵只用于判断，不会渲染到界面。
            if "includes(" in line or "RegExp" in line:
                continue
            for match in STRING.finditer(line):
                value = match.group(0)
                found = "".join(sorted(set(HANGUL.findall(value))))
                if found:
                    offenders.append((path, line_no, found))

    for folder in (root / "web" / "out", root / "server" / "webui" / "dist"):
        if not folder.is_dir():
            continue
        for path in folder.rglob("*.html"):
            text = path.read_text(encoding="utf-8", errors="ignore")
            found = "".join(sorted(set(HANGUL.findall(text))))
            if found:
                offenders.append((path, 0, found))
    return offenders


def main() -> int:
    root = source_root()
    offenders = check_source(root)
    if offenders:
        print("中文语言门禁失败：发现韩文用户可见字符串")
        for path, line, found in offenders:
            suffix = f":{line}" if line else ""
            print(f"  {path.relative_to(root)}{suffix}：{found}")
        return 1
    print("中文语言门禁通过：未发现韩文用户可见字符串")
    return 0


if __name__ == "__main__":
    sys.exit(main())
