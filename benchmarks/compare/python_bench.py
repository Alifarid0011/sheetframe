#!/usr/bin/env python3
"""Eager pandas/Polars operations; imports, construction and checks are untimed."""
import argparse
import gc
import hashlib
import importlib.util
import json
import os
import platform
import statistics
import time

OPERATIONS = ("select", "filter", "group_sum", "stable_sort", "pipeline")


def expected(n, operation):
    digest = hashlib.sha256()
    count = 0

    def row(i):
        nonlocal count
        digest.update(f"g{i % 20:02d}\t{i % 1000}\t{str((i // 1000) % 2 == 0).lower()}\n".encode())
        count += 1

    columns = ["group", "amount", "flag"]
    if operation == "select":
        columns = ["amount"]
        for i in range(n):
            digest.update(f"{i % 1000}\n".encode())
        count = n
    elif operation == "filter":
        for i in range(n):
            if i % 1000 >= 500:
                row(i)
    elif operation == "stable_sort":
        for amount in range(999, -1, -1):
            for i in range(amount, n, 1000):
                row(i)
    else:
        columns = ["group", "total"]
        sums = [0] * 20
        for i in range(n):
            if operation == "group_sum" or i % 1000 >= 500:
                sums[i % 20] += i % 1000
        order = range(20)
        if operation == "pipeline":
            order = sorted(order, key=lambda key: -sums[key])
        for key in order:
            digest.update(f"g{key:02d}\t{sums[key]}\n".encode())
        count = 20
    return count, columns, digest.hexdigest()


def checksum(rows):
    digest = hashlib.sha256()
    for row in rows:
        fields = []
        for value in row:
            if isinstance(value, str):
                fields.append(value)
            elif isinstance(value, bool):
                fields.append(str(value).lower())
            else:
                assert float(value).is_integer(), value
                fields.append(format(value, ".0f"))
        digest.update(("\t".join(fields) + "\n").encode())
    return digest.hexdigest()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--engine", choices=("pandas", "pandas-object", "polars"), required=True)
    parser.add_argument("--rows", default="100000,1000000")
    parser.add_argument("--repeats", type=int, default=7)
    parser.add_argument("--warmups", type=int, default=2)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    if args.repeats < 1 or args.warmups < 0:
        parser.error("repeats must be positive and warmups nonnegative")
    import numpy as np
    if args.engine.startswith("pandas"):
        import pandas as lib
    else:
        import polars as lib
    metadata = {
        "python": platform.python_version(), "os": platform.platform(),
        "processor": platform.processor(), "logical_cpus": os.cpu_count(),
        "numpy": np.__version__, "version": lib.__version__,
        "optional_pyarrow_installed": importlib.util.find_spec("pyarrow") is not None,
        "warmups": args.warmups, "repeats": args.repeats, "check_only": args.check,
        "thread_environment": {key: os.environ.get(key) for key in (
            "OMP_NUM_THREADS", "OPENBLAS_NUM_THREADS", "MKL_NUM_THREADS",
            "NUMEXPR_NUM_THREADS", "POLARS_MAX_THREADS")},
        "timing": "native eager operation result; checksum/export/GC before sample excluded; normal GC during operation enabled",
    }
    if args.engine.startswith("pandas"):
        metadata["pandas_copy_on_write"] = str(lib.options.mode.copy_on_write)
    else:
        metadata["polars_thread_pool_size"] = lib.thread_pool_size()
    report = {"metadata": metadata, "results": []}
    for n in map(int, args.rows.split(",")):
        assert n >= 1000, "Use at least 1000 rows to include all groups and amounts"
        idx = np.arange(n)
        groups = np.array([f"g{i % 20:02d}" for i in range(n)])
        amounts = (idx % 1000).astype(np.float64)
        flags = ((idx // 1000) % 2) == 0
        data = {"group": groups, "amount": amounts, "flag": flags}
        if args.engine == "pandas-object":
            data["group"] = lib.Series(groups, dtype=object)
        frame = lib.DataFrame(data)
        if args.engine.startswith("pandas"):
            metadata["dtypes"] = {str(k): str(v) for k, v in frame.dtypes.items()}
            metadata["pandas_string_storage"] = getattr(frame["group"].dtype, "storage", "object")
        else:
            metadata["dtypes"] = {str(k): str(v) for k, v in frame.schema.items()}

        def operate(operation):
            if args.engine.startswith("pandas"):
                if operation == "select":
                    return frame[["amount"]]
                if operation == "filter":
                    return frame.loc[frame["amount"] >= 500.0]
                if operation == "stable_sort":
                    return frame.sort_values("amount", ascending=False, kind="stable")
                source = frame if operation == "group_sum" else frame.loc[frame["amount"] >= 500.0]
                grouped = source.groupby("group", sort=False, as_index=False, observed=True)["amount"].sum().rename(columns={"amount": "total"})
                return grouped if operation == "group_sum" else grouped.sort_values("total", ascending=False, kind="stable")
            if operation == "select":
                return frame.select("amount")
            if operation == "filter":
                return frame.filter(lib.col("amount") >= 500.0)
            if operation == "stable_sort":
                return frame.sort("amount", descending=True, maintain_order=True)
            source = frame if operation == "group_sum" else frame.filter(lib.col("amount") >= 500.0)
            grouped = source.group_by("group", maintain_order=True).agg(lib.col("amount").sum().alias("total"))
            return grouped if operation == "group_sum" else grouped.sort("total", descending=True, maintain_order=True)

        for operation in OPERATIONS:
            wanted_rows, wanted_columns, wanted_hash = expected(n, operation)

            def validate(output):
                assert output.shape == (wanted_rows, len(wanted_columns)), (operation, output.shape)
                assert list(output.columns) == wanted_columns
                rows = output.itertuples(index=False, name=None) if args.engine.startswith("pandas") else output.iter_rows()
                got = checksum(rows)
                assert got == wanted_hash, (args.engine, operation, n, got, wanted_hash)

            samples = []
            if args.check:
                validate(operate(operation))
            else:
                for _ in range(args.warmups):
                    output = operate(operation)
                    validate(output)
                    del output
                for _ in range(args.repeats):
                    gc.collect()
                    start = time.perf_counter_ns()
                    output = operate(operation)
                    elapsed = time.perf_counter_ns() - start
                    samples.append(elapsed)
                    validate(output)
                    del output
            result = {
                "engine": args.engine, "version": lib.__version__, "input_rows": n,
                "operation": operation, "output_rows": wanted_rows,
                "columns": wanted_columns, "sha256": wanted_hash, "status": "ok",
            }
            if samples:
                result.update(samples_ns=samples, median_ns=statistics.median(samples))
            report["results"].append(result)
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
