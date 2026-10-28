#!/usr/bin/env python3
"""Sequential reproducible comparison runner; builds happen before measurement."""
import argparse
import csv
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import time

HERE = Path(__file__).resolve().parent
THREAD_VARS = ("OMP_NUM_THREADS", "OPENBLAS_NUM_THREADS", "MKL_NUM_THREADS", "NUMEXPR_NUM_THREADS", "POLARS_MAX_THREADS")


def source_identity(path):
    files = sorted(path.glob("*.go")) + [path / "go.mod"]
    digest = hashlib.sha256()
    for file in files:
        if file.name.endswith("_test.go"):
            continue
        digest.update(file.name.encode() + b"\0" + file.read_bytes() + b"\0")
    def git(*args):
        proc = subprocess.run(["git", "-C", str(path), *args], text=True, capture_output=True)
        return proc.stdout.strip() if proc.returncode == 0 else None
    return {"path": str(path), "core_source_sha256": digest.hexdigest(), "git_head": git("rev-parse", "HEAD"), "git_status": git("status", "--porcelain")}


def execute(command, env, *, cwd=HERE):
    print("Running: " + " ".join(map(str, command)), file=sys.stderr, flush=True)
    proc = subprocess.run(list(map(str, command)), cwd=cwd, env=env, text=True, capture_output=True)
    if proc.returncode:
        print(proc.stderr, file=sys.stderr)
        proc.check_returncode()
    return proc.stdout


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline", type=Path, help="directory containing the v0.1.0 core snapshot")
    parser.add_argument("--current", type=Path, default=HERE.parent.parent)
    parser.add_argument("--rows", default="100000,1000000")
    parser.add_argument("--repeats", type=int, default=7)
    parser.add_argument("--warmups", type=int, default=2)
    parser.add_argument("--mode", choices=("single", "default"), default="single")
    parser.add_argument("--cpu", type=int, help="pin runner and children to this allowed CPU on Linux")
    parser.add_argument("--output", type=Path)
    parser.add_argument("--prepare", action="store_true", help="build Go executables and inspect Python versions without running operations")
    parser.add_argument("--check", action="store_true", help="validate only, without recording any timings")
    parser.add_argument("--reuse-builds", action="store_true", help="skip compilation; rejects stale source fingerprints")
    parser.add_argument("--engines", default="baseline,current,qframe,pandas,pandas-object,polars")
    args = parser.parse_args()
    if args.cpu is not None:
        if not hasattr(os, "sched_getaffinity") or args.cpu not in os.sched_getaffinity(0):
            parser.error("--cpu must name an allowed Linux CPU")
        os.sched_setaffinity(0, {args.cpu})
    engines = args.engines.split(",")
    unknown = set(engines) - {"baseline", "current", "qframe", "pandas", "pandas-object", "polars"}
    if unknown:
        parser.error(f"Unknown engines: {unknown}")
    if "baseline" in engines and args.baseline is None:
        parser.error("--baseline is required when baseline is selected")
    if args.repeats < 1 or args.warmups < 0:
        parser.error("repeats must be positive; warmups nonnegative")
    env = os.environ.copy()
    if args.mode == "single":
        env.update({name: "1" for name in THREAD_VARS})
        env["GOMAXPROCS"] = "1"
    else:
        for name in THREAD_VARS + ("GOMAXPROCS",):
            env.pop(name, None)
    env.setdefault("GOCACHE", str(HERE / ".build" / "go-cache"))
    python = HERE / ".venv" / "bin" / "python"
    if not python.exists():
        parser.error("Missing .venv: follow the pinned install commands in README.md")
    versions = json.loads(execute([python, "-c", "import json,numpy,pandas,polars;print(json.dumps({m.__name__:m.__version__ for m in (numpy,pandas,polars)}))"], env))
    builds = {}
    identities = {}
    variants = (["baseline"] if "baseline" in engines else []) + (["current"] if set(engines) & {"current", "qframe"} else [])
    harness_hash = hashlib.sha256((HERE / "main.go").read_bytes() + (HERE / "go.mod").read_bytes() + (HERE / "go.sum").read_bytes()).hexdigest()
    for variant in variants:
        source = (args.baseline if variant == "baseline" else args.current).resolve()
        identity = source_identity(source)
        identities[variant] = identity
        directory = HERE / ".build" / variant
        directory.mkdir(parents=True, exist_ok=True)
        fingerprint = {"source": identity["core_source_sha256"], "harness": harness_hash}
        fingerprint_path = directory / "fingerprint.json"
        binary = directory / "compare"
        if args.reuse_builds:
            if not binary.exists() or not fingerprint_path.exists() or json.loads(fingerprint_path.read_text()) != fingerprint:
                parser.error(f"{variant} build is missing/stale; rerun without --reuse-builds")
        else:
            shutil.copy2(HERE / "main.go", directory / "main.go")
            shutil.copy2(HERE / "go.sum", directory / "go.sum")
            module = (HERE / "go.mod").read_text().replace("=> ../..", "=> " + str(source))
            (directory / "go.mod").write_text(module)
            execute(["go", "build", "-mod=mod", "-o", binary, "."], env, cwd=directory)
            fingerprint_path.write_text(json.dumps(fingerprint, indent=2) + "\n")
        builds[variant] = binary
    if args.prepare:
        print(json.dumps({"python_versions": versions, "builds": {k: str(v) for k, v in builds.items()}, "sources": identities}, indent=2))
        return
    output = args.output or HERE / "results" / dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    if output.exists():
        parser.error(f"Output already exists: {output}; choose a fresh directory")
    output.mkdir(parents=True)
    cpu = next((line.split(":", 1)[1].strip() for line in Path("/proc/cpuinfo").read_text().splitlines() if line.startswith("model name")), "unknown") if Path("/proc/cpuinfo").exists() else platform.processor()
    metadata = {
        "started_utc": dt.datetime.now(dt.timezone.utc).isoformat(), "cpu": cpu,
        "platform": platform.platform(), "logical_cpus": os.cpu_count(),
        "mode": args.mode, "cpu_affinity": sorted(os.sched_getaffinity(0)) if hasattr(os, "sched_getaffinity") else None, "thread_environment": {key: env.get(key) for key in THREAD_VARS + ("GOMAXPROCS",)},
        "python_versions": versions, "sources": identities, "go_harness_sha256": harness_hash,
        "harness_sha256": hashlib.sha256(b"".join((HERE / name).read_bytes() for name in ("main.go", "go.mod", "go.sum", "run.py", "python_bench.py", "requirements.txt"))).hexdigest(),
        "rows": args.rows, "repeats": args.repeats, "warmups": args.warmups,
        "check_only": args.check, "engine_order": engines, "load_before": os.getloadavg() if hasattr(os, "getloadavg") else None,
        "command": sys.argv, "timing_scope": "native eager results; input loading, generation, process startup, checksum and validation excluded",
        "dataset": "null-free: group=g(i%20), amount=float64(i%1000), flag=((i//1000)%2==0); three columns; no categoricals",
    }
    (output / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")
    reports = []
    shared = ["--rows", args.rows, "--repeats", str(args.repeats), "--warmups", str(args.warmups)]
    if args.check:
        shared.append("--check")
    for engine in engines:
        if engine in ("baseline", "current", "qframe"):
            variant = "baseline" if engine == "baseline" else "current"
            native = "qframe" if engine == "qframe" else "sheetframe"
            label = "qframe" if engine == "qframe" else "sheetframe-" + engine
            command = [builds[variant], "--engine", native, "--label", label] + shared
        else:
            command = [python, HERE / "python_bench.py", "--engine", engine] + shared
        started = time.time()
        text = execute(command, env)
        parsed = json.loads(text)
        parsed["metadata"]["process_wall_seconds_including_untimed_work"] = time.time() - started
        (output / f"{engine}.json").write_text(json.dumps(parsed, indent=2) + "\n")
        reports.extend(parsed["results"])
    # Reject cross-engine differences even if an individual oracle was changed.
    digests = {}
    for result in reports:
        if result["status"] != "ok":
            continue
        key = (result["input_rows"], result["operation"])
        if key in digests and digests[key] != result["sha256"]:
            raise AssertionError(f"Cross-engine checksum mismatch: {key}")
        digests[key] = result["sha256"]
    with (output / "summary.csv").open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=["engine", "version", "input_rows", "operation", "status", "median_ns", "median_ms", "output_rows", "sha256", "note"])
        writer.writeheader()
        for result in reports:
            row = {key: result.get(key, "") for key in writer.fieldnames}
            if "median_ns" in result:
                row["median_ms"] = result["median_ns"] / 1e6
            writer.writerow(row)
    with (output / "samples.csv").open("w", newline="") as handle:
        writer = csv.writer(handle)
        writer.writerow(["engine", "input_rows", "operation", "sample", "elapsed_ns"])
        for result in reports:
            for index, ns in enumerate(result.get("samples_ns", []), start=1):
                writer.writerow([result["engine"], result["input_rows"], result["operation"], index, ns])
    metadata.update(finished_utc=dt.datetime.now(dt.timezone.utc).isoformat(), load_after=os.getloadavg() if hasattr(os, "getloadavg") else None, all_checksums_match=True)
    (output / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")
    print(f"Validated results written to {output}")


if __name__ == "__main__":
    main()
