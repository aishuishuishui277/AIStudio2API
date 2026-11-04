#!/usr/bin/env python3
"""Use Mozilla's own source-package Cargo filter for the standalone JS build."""
import ast
import enum
import pathlib
import sys

source = pathlib.Path(sys.argv[1]).resolve()
packager = source / "js/src/make-source-package.py"
tree = ast.parse(packager.read_text(encoding="utf-8"))
names = {"is_mozjs_cargo_member", "is_mozjs_crates_io_local_patch", "copy_cargo_toml"}
functions = [node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name in names]
if len(functions) != len(names):
    raise SystemExit("Unexpected SpiderMonkey packager; refusing to modify Cargo.toml")
namespace = {"enum": enum, "topsrc_dir": source, "target_dir": source}
module = ast.Module(body=functions, type_ignores=[])
exec(compile(ast.fix_missing_locations(module), str(packager), "exec"), namespace)
namespace["copy_cargo_toml"]()
