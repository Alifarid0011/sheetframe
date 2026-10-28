#!/usr/bin/env python3
"""Render the public benchmark table from a complete, validated comparison run."""
import argparse
import csv
import html
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ENGINES = {
    "sheetframe-baseline": "Sheetframe 0.1",
    "sheetframe-current": "Sheetframe 0.2",
    "pandas": "pandas default",
    "pandas-object": "pandas object",
    "polars": "Polars",
    "qframe": "QFrame",
}
OPERATIONS = {
    "select": "Select one column",
    "filter": "Filter rows",
    "group_sum": "Group + sum",
    "stable_sort": "Stable numeric sort",
    "pipeline": "Filter → group → sort",
}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("results", type=Path)
    parser.add_argument("--output", type=Path, default=ROOT / "docs/benchmarks.html")
    args = parser.parse_args()
    metadata = json.loads((args.results / "metadata.json").read_text())
    if not metadata.get("all_checksums_match") or metadata.get("check_only"):
        raise ValueError("A complete timed run with matching checksums is required")
    with (args.results / "summary.csv").open() as handle:
        records = list(csv.DictReader(handle))
    lookup = {(int(r["input_rows"]), r["operation"], r["engine"]): r for r in records}
    tables = []
    for size in sorted({int(r["input_rows"]) for r in records}):
        body = []
        for operation, label in OPERATIONS.items():
            cells = []
            for engine in ENGINES:
                result = lookup[(size, operation, engine)]
                if result["status"] == "skipped":
                    value = '<span title="Incompatible stable-sort semantics">—</span>'
                elif result["status"] == "ok":
                    ns = float(result["median_ns"])
                    value = f"{ns / 1e6:,.2f} ms" if ns >= 1e6 else f"{ns / 1e3:,.2f} µs"
                else:
                    raise ValueError(f"Incomplete result: {result}")
                current = ' class="current"' if engine == "sheetframe-current" else ""
                cells.append(f"<td{current}>{value}</td>")
            body.append(f'<tr><th scope="row">{html.escape(label)}</th>{"".join(cells)}</tr>')
        headers = "".join(f'<th scope="col">{label}</th>' for label in ENGINES.values())
        tables.append(f'<section><h2>{size:,} rows</h2><div class="comparison-scroll" tabindex="0" role="region" aria-label="{size:,} row benchmark results"><table><caption>Median operation time. Lower is faster.</caption><thead><tr><th scope="col">Operation</th>{headers}</tr></thead><tbody>{"".join(body)}</tbody></table></div></section>')
    raw_path = args.results.resolve().relative_to(ROOT).as_posix()
    raw_url = "https://github.com/Alifarid0011/sheetframe/tree/main/" + raw_path
    cpu = html.escape(metadata["cpu"])
    date = html.escape(metadata["started_utc"][:10])
    versions = html.escape(", ".join(f"{name} {version}" for name, version in metadata["python_versions"].items()))
    threads = "one thread per engine" if metadata["mode"] == "single" else "engine default thread counts"
    affinity = html.escape(str(metadata.get("cpu_affinity", "unrestricted")))
    warmups, repeats = int(metadata["warmups"]), int(metadata["repeats"])
    qframe = json.loads((args.results / "qframe.json").read_text())["results"][0]["version"]
    page = f'''<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Sheetframe — measured performance</title><meta name="description" content="Reproducible native benchmarks against pandas, Polars and QFrame, with raw samples and explicit limitations."><link rel="stylesheet" href="styles.css"><style>
.comparison-hero{{padding:64px 0 32px;max-width:850px}}h1{{font:650 clamp(36px,5vw,64px)/1.1 Manrope,sans-serif;letter-spacing:-2px;margin:0 0 24px}}.comparison-hero p,.method p,.method li{{line-height:1.8;color:var(--muted);font-size:14px}}main section{{margin-bottom:48px}}.comparison-scroll{{overflow-x:auto;border:1px solid var(--line);border-radius:8px;background:white;margin-top:22px}}table{{border-collapse:collapse;width:100%;white-space:nowrap;font-variant-numeric:tabular-nums;font-size:13px}}caption{{text-align:left;padding:16px;color:var(--muted);font-size:12px}}th,td{{padding:16px;text-align:right;border-top:1px solid var(--line)}}th:first-child{{text-align:left}}thead th{{font-weight:600;font-size:11px}}td.current{{background:#eef6d5;font-weight:700;color:var(--navy)}}.method{{max-width:850px}}.method a{{text-decoration:underline;text-underline-offset:3px}}.compare-meta{{border-left:3px solid var(--lime);padding-left:18px}}@media(max-width:700px){{.site-header nav{{gap:12px}}th,td{{padding:12px}}}}
</style></head><body><header class="site-header shell"><a class="wordmark" href="./">sheetframe</a><nav aria-label="Main"><a href="./">Playground</a><a class="repo-link" href="{raw_url}">Raw results ↗</a></nav></header><main class="shell"><div class="comparison-hero"><p class="eyebrow">MEASURED. REPRODUCIBLE. WORKLOAD-SPECIFIC.</p><h1>Performance you<br>can inspect.</h1><p>Native, eager operations on identical tables. Every result is checked against an independent row-by-row checksum. These measurements include faster and slower cases; they are not a claim of general superiority.</p><p class="compare-meta">{cpu} · {threads}, CPU affinity {affinity} · {date}<br>{warmups} warmups · {repeats} timed samples per case · {versions} · QFrame {html.escape(qframe)}</p></div>{"".join(tables)}<section class="method"><h2>What these numbers mean</h2><p>The input has three columns: 20 repeated string groups, float64 amounts from 0 to 999, and boolean flags. Input creation, imports, startup, result conversion and checksum validation are outside the timer. Excel I/O is not measured. All operations produce native DataFrame results; selection can share buffers and filters can use indexed views.</p><p>pandas is shown with both its default Python-backed string dtype and an explicit object string column. Optional PyArrow is absent. Polars uses eager operations and stable ordering. QFrame’s incompatible unstable sort is omitted; its group case includes a timed sort to match group order for this dataset.</p><p>Sheetframe 0.2 replaces JSON key construction with typed hashing and streamed aggregates. Large numeric columns use stable radix sorting, which consumes extra temporary memory. The native allocation benchmark reports that tradeoff separately.</p><p>A single synthetic, null-free workload cannot establish the fastest library for every job. Thread count, strings, cardinality, nulls, data distribution and hardware can change the result. <a href="https://github.com/Alifarid0011/sheetframe/blob/main/docs/BENCHMARKS.md">Read the methodology and native allocation results</a>, <a href="https://github.com/Alifarid0011/sheetframe/tree/main/benchmarks/compare">rerun the benchmark</a>, or <a href="{raw_url}">inspect every raw sample</a>.</p></section></main><footer class="site-footer shell"><a class="wordmark" href="./">sheetframe<span class="footer-by">by alifarid</span></a><a href="https://github.com/Alifarid0011/sheetframe">Source on GitHub ↗</a></footer></body></html>'''
    args.output.write_text(page)
    print(args.output)


if __name__ == "__main__":
    main()
