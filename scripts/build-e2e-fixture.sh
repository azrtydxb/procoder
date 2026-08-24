#!/usr/bin/env bash
# Builds the end-to-end fixture repository: a repository procoder has never
# seen, carrying at least one file for every extension in the tool table,
# its own tests, a CI workflow that predates procoder, docs and dependency
# manifests.
#
# Built by script rather than committed, for two reasons. The broken pass
# plants secrets and vulnerable manifests in it, and committing those into
# procoder's own repository would trip procoder's own gate — correctly. And
# a script means a finding can be reproduced from `git init`, rather than
# from whatever state somebody happened to be in.
#
# Usage: build-e2e-fixture.sh [dir]        (default: $TMPDIR/procoder-e2e-fixture)
#
# Idempotent: an existing fixture is removed and rebuilt, never layered on.
set -euo pipefail

DEST="${1:-${TMPDIR:-/tmp}/procoder-e2e-fixture}"
DEST="${DEST%/}"

case "$DEST" in
"" | / | /Users | /home | "$HOME")
	echo "refusing to build the fixture at $DEST" >&2
	exit 2
	;;
esac

# A rebuild must produce the same tree as the first build, so every commit
# is stamped with a fixed identity and a fixed clock rather than today's.
export GIT_AUTHOR_NAME="Fixture Author" GIT_AUTHOR_EMAIL="fixture@example.invalid"
export GIT_COMMITTER_NAME="$GIT_AUTHOR_NAME" GIT_COMMITTER_EMAIL="$GIT_AUTHOR_EMAIL"
export GIT_AUTHOR_DATE="2026-01-02T03:04:05+00:00"
export GIT_COMMITTER_DATE="$GIT_AUTHOR_DATE"

rm -rf "$DEST"
mkdir -p "$DEST"
cd "$DEST"

f() {
	mkdir -p "$(dirname "$1")"
	cat >"$1"
}

# ---------------------------------------------------------------- Go
f go.mod <<'EOF'
module example.invalid/fixture

go 1.24
EOF

f greet/greet.go <<'EOF'
// Package greet builds the greeting the fixture's binary prints.
package greet

import "strings"

// Greet returns a greeting for name, or a generic one when name is blank.
func Greet(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Hello, stranger!"
	}
	return "Hello, " + name + "!"
}
EOF

f greet/greet_test.go <<'EOF'
package greet

import "testing"

func TestGreetUsesTheName(t *testing.T) {
	if got := Greet("Ada"); got != "Hello, Ada!" {
		t.Errorf("Greet(Ada) = %q", got)
	}
}

func TestBlankNameFallsBackRatherThanGreetingNobody(t *testing.T) {
	if got := Greet("   "); got != "Hello, stranger!" {
		t.Errorf("Greet(blank) = %q", got)
	}
}
EOF

f main.go <<'EOF'
// Command fixture prints a greeting. It exists so the fixture repository
// builds, tests and runs like a real project rather than a pile of samples.
package main

import (
	"fmt"
	"os"

	"example.invalid/fixture/greet"
)

func main() {
	name := ""
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	fmt.Println(greet.Greet(name))
}
EOF

# ---------------------------------------------------------------- Python
f pyproject.toml <<'EOF'
[project]
name = "fixture"
version = "0.1.0"
requires-python = ">=3.11"
dependencies = []

[tool.ruff]
line-length = 88
EOF

f py/greet.py <<'EOF'
"""Greeting helpers, the Python half of the fixture."""


def greet(name: str) -> str:
    """Return a greeting for ``name``, or a generic one when it is blank."""
    name = name.strip()
    if not name:
        return "Hello, stranger!"
    return f"Hello, {name}!"
EOF

f py/greet.pyi <<'EOF'
def greet(name: str) -> str: ...
EOF

f py/test_greet.py <<'EOF'
from greet import greet


def test_greet_uses_the_name() -> None:
    assert greet("Ada") == "Hello, Ada!"


def test_blank_name_falls_back() -> None:
    assert greet("   ") == "Hello, stranger!"
EOF

# ---------------------------------------------------------------- Rust
f Cargo.toml <<'EOF'
[package]
name = "fixture"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = "1.0.210"
EOF

f src/lib.rs <<'EOF'
//! Greeting helpers, the Rust half of the fixture.

/// Returns a greeting for `name`, or a generic one when it is blank.
pub fn greet(name: &str) -> String {
    let name = name.trim();
    if name.is_empty() {
        return "Hello, stranger!".to_string();
    }
    format!("Hello, {name}!")
}

#[cfg(test)]
mod tests {
    use super::greet;

    #[test]
    fn uses_the_name() {
        assert_eq!(greet("Ada"), "Hello, Ada!");
    }

    #[test]
    fn blank_name_falls_back() {
        assert_eq!(greet("   "), "Hello, stranger!");
    }
}
EOF

# ---------------------------------------------------------------- C / C++
f c/greet.h <<'EOF'
#ifndef FIXTURE_GREET_H
#define FIXTURE_GREET_H

/* Writes a greeting for name into out, which must hold at least n bytes. */
void fixture_greet(const char *name, char *out, unsigned long n);

#endif /* FIXTURE_GREET_H */
EOF

f c/greet.c <<'EOF'
#include "greet.h"

#include <stdio.h>
#include <string.h>

void fixture_greet(const char *name, char *out, unsigned long n) {
  if (name == NULL || name[0] == '\0') {
    snprintf(out, n, "Hello, stranger!");
    return;
  }
  snprintf(out, n, "Hello, %s!", name);
}
EOF

f cpp/greet.hpp <<'EOF'
#pragma once

#include <string>

namespace fixture {

// Returns a greeting for name, or a generic one when it is empty.
std::string greet(const std::string &name);

} // namespace fixture
EOF

f cpp/greet.cpp <<'EOF'
#include "greet.hpp"

namespace fixture {

std::string greet(const std::string &name) {
  if (name.empty()) {
    return "Hello, stranger!";
  }
  return "Hello, " + name + "!";
}

} // namespace fixture
EOF

f cpp/greet.cc <<'EOF'
#include "greet.hpp"

#include <iostream>

int main() {
  std::cout << fixture::greet("Ada") << '\n';
  return 0;
}
EOF

f cpp/legacy.cxx <<'EOF'
#include "greet.hpp"

// The .cxx extension is in procoder's table, so the fixture carries one.
namespace fixture {

std::string shout(const std::string &name) { return greet(name) + "!!"; }

} // namespace fixture
EOF

# ---------------------------------------------------------------- shell
f sh/greet.sh <<'EOF'
#!/usr/bin/env bash
# Prints a greeting. The shell half of the fixture.
set -euo pipefail

name="${1:-}"
if [ -z "$name" ]; then
	echo "Hello, stranger!"
else
	echo "Hello, $name!"
fi
EOF
chmod +x sh/greet.sh

f sh/lib.bash <<'EOF'
#!/usr/bin/env bash
# Sourced helpers. The .bash extension is in procoder's table.

fixture_upper() {
	printf '%s\n' "${1:-}" | tr '[:lower:]' '[:upper:]'
}
EOF

# ---------------------------------------------------------------- Java
f pom.xml <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>invalid.example</groupId>
  <artifactId>fixture</artifactId>
  <version>0.1.0</version>
  <properties>
    <maven.compiler.release>17</maven.compiler.release>
  </properties>
</project>
EOF

f src/main/java/invalid/example/Greet.java <<'EOF'
package invalid.example;

/** Greeting helpers, the Java half of the fixture. */
public final class Greet {
  private Greet() {}

  /** Returns a greeting for name, or a generic one when it is blank. */
  public static String greet(String name) {
    if (name == null || name.isBlank()) {
      return "Hello, stranger!";
    }
    return "Hello, " + name.strip() + "!";
  }
}
EOF

# ---------------------------------------------------------------- Kotlin
f kt/Greet.kt <<'EOF'
package invalid.example

/** Returns a greeting for [name], or a generic one when it is blank. */
fun greet(name: String): String {
    val trimmed = name.trim()
    if (trimmed.isEmpty()) return "Hello, stranger!"
    return "Hello, $trimmed!"
}
EOF

f build.gradle.kts <<'EOF'
plugins {
    kotlin("jvm") version "2.0.21"
}

repositories {
    mavenCentral()
}
EOF

# ---------------------------------------------------------------- Swift
f swift/Greet.swift <<'EOF'
/// Returns a greeting for `name`, or a generic one when it is blank.
public func greet(_ name: String) -> String {
    let trimmed = name.trimmingCharacters(in: .whitespaces)
    if trimmed.isEmpty {
        return "Hello, stranger!"
    }
    return "Hello, \(trimmed)!"
}
EOF

# ---------------------------------------------------------------- Ruby
f Gemfile <<'EOF'
source "https://rubygems.org"

gem "rake", "13.2.1"
EOF

f rb/greet.rb <<'EOF'
# frozen_string_literal: true

# Greeting helpers, the Ruby half of the fixture.
module Greet
  def self.call(name)
    trimmed = name.to_s.strip
    return "Hello, stranger!" if trimmed.empty?

    "Hello, #{trimmed}!"
  end
end
EOF

f Rakefile.rake <<'EOF'
# frozen_string_literal: true

desc "Print the fixture greeting"
task :greet do
  require_relative "rb/greet"
  puts Greet.call("Ada")
end
EOF

# ---------------------------------------------------------------- Dart
f pubspec.yaml <<'EOF'
name: fixture
description: The fixture repository's Dart half.
environment:
  sdk: ">=3.0.0 <4.0.0"
EOF

f dart/greet.dart <<'EOF'
/// Returns a greeting for [name], or a generic one when it is blank.
String greet(String name) {
  final trimmed = name.trim();
  if (trimmed.isEmpty) {
    return 'Hello, stranger!';
  }
  return 'Hello, $trimmed!';
}
EOF

# ---------------------------------------------------------------- C#
f cs/Fixture.csproj <<'EOF'
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <Nullable>enable</Nullable>
  </PropertyGroup>
</Project>
EOF

f cs/Greet.cs <<'EOF'
namespace Fixture;

/// <summary>Greeting helpers, the C# half of the fixture.</summary>
public static class Greet
{
    /// <summary>Returns a greeting for <paramref name="name" />.</summary>
    public static string Call(string? name)
    {
        var trimmed = name?.Trim() ?? string.Empty;
        return trimmed.Length == 0 ? "Hello, stranger!" : $"Hello, {trimmed}!";
    }
}
EOF

# ---------------------------------------------------------------- PHP
f composer.json <<'EOF'
{
  "name": "example/fixture",
  "description": "The fixture repository's PHP half.",
  "require": {
    "php": ">=8.2"
  }
}
EOF

f php/Greet.php <<'EOF'
<?php

declare(strict_types=1);

/** Returns a greeting for $name, or a generic one when it is blank. */
function fixture_greet(string $name): string
{
    $trimmed = trim($name);
    if ($trimmed === "") {
        return "Hello, stranger!";
    }

    return "Hello, {$trimmed}!";
}
EOF

# ------------------------------------------------- JS / TS / web (prettier)
f package.json <<'EOF'
{
  "name": "fixture",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "test": "node --test web/*.test.js"
  }
}
EOF

f web/greet.js <<'EOF'
export function greet(name) {
  const trimmed = String(name ?? "").trim();
  return trimmed === "" ? "Hello, stranger!" : `Hello, ${trimmed}!`;
}
EOF

f web/greet.test.js <<'EOF'
import assert from "node:assert/strict";
import test from "node:test";

import { greet } from "./greet.js";

test("greet uses the name", () => {
  assert.equal(greet("Ada"), "Hello, Ada!");
});

test("a blank name falls back", () => {
  assert.equal(greet("   "), "Hello, stranger!");
});
EOF

f web/greet.mjs <<'EOF'
export { greet } from "./greet.js";
EOF

f web/legacy.cjs <<'EOF'
"use strict";

module.exports.greet = function greet(name) {
  const trimmed = String(name || "").trim();
  return trimmed === "" ? "Hello, stranger!" : "Hello, " + trimmed + "!";
};
EOF

f web/greet.ts <<'EOF'
export function greet(name: string): string {
  const trimmed = name.trim();
  return trimmed === "" ? "Hello, stranger!" : `Hello, ${trimmed}!`;
}
EOF

f web/greet.mts <<'EOF'
export type Greeter = (name: string) => string;
EOF

f web/legacy.cts <<'EOF'
export type LegacyGreeter = (name: string) => string;
EOF

f web/Greeting.jsx <<'EOF'
export function Greeting({ name }) {
  return <p>Hello, {name || "stranger"}!</p>;
}
EOF

f web/Greeting.tsx <<'EOF'
export function Greeting({ name }: { name?: string }) {
  return <p>Hello, {name || "stranger"}!</p>;
}
EOF

f web/greet.json <<'EOF'
{
  "greeting": "Hello",
  "fallback": "stranger"
}
EOF

f web/greet.css <<'EOF'
.greeting {
  color: #222;
  font-family: system-ui, sans-serif;
}
EOF

f web/greet.scss <<'EOF'
$ink: #222;

.greeting {
  color: $ink;

  &:hover {
    text-decoration: underline;
  }
}
EOF

f web/index.html <<'EOF'
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>Fixture</title>
    <link rel="stylesheet" href="greet.css" />
  </head>
  <body>
    <p class="greeting">Hello, stranger!</p>
  </body>
</html>
EOF

f web/greet.yaml <<'EOF'
greeting: Hello
fallback: stranger
EOF

f web/greet.yml <<'EOF'
greeting: Hello
fallback: stranger
EOF

# ---------------------------------------------------------------- docs
f README.md <<'EOF'
# fixture 0.1.0

[![ci](https://img.shields.io/badge/ci-passing-brightgreen)](.github/workflows/ci.yml)
[![license](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

A repository that exists to be checked. It carries one file for every
extension procoder's tool table names, a test suite in several runners, a
CI workflow, and dependency manifests — so that running procoder against
it answers what procoder says about a project that is not its own.

## Quick start

```sh
go run . Ada
```

See [the docs](docs/usage.md) for what it does, which is print a greeting.

Built by `scripts/build-e2e-fixture.sh` in the procoder repository. It is
not maintained by hand and anything committed to it will be lost on the
next rebuild.
EOF

f CHANGELOG.md <<'EOF'
# Changelog

## 0.1.0

- The fixture greets a name, or a stranger when given none, in every
  language procoder claims to format.
EOF

f LICENSE <<'EOF'
MIT License

Copyright (c) 2026 Fixture Author

Permission is hereby granted, free of charge, to any person obtaining a
copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND.
EOF

f docs/usage.md <<'EOF'
# Usage

Every language in this repository implements the same function: given a
name, return a greeting; given nothing, greet a stranger.

```sh
go run . Ada
./sh/greet.sh Ada
```

The [README](../README.md) says why the repository exists. The upstream
project is [procoder](https://github.com/azrtydxb/procoder).
EOF

# ---------------------------------------------------------------- CI
f .github/workflows/ci.yml <<'EOF'
name: ci

on:
  push:
    branches: [main]
  pull_request:

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  test:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2
      - uses: actions/setup-go@41dfa10bad2bb2ae585af6ee5bb4d7d973ad74ed # v5.1.0
        with:
          go-version: "1.24"
      - run: go test ./...
EOF

f .gitignore <<'EOF'
/target/
/node_modules/
/bin/
__pycache__/
*.class
*.test
# generated by the toolchain, and not reproducible between builds
/package-lock.json
/Cargo.lock
# what procoder writes into a repository it governs
.procoder/index/
.procoder/state/
.lycheecache
EOF

git init -q -b main
git add -A
git commit -q -m "the fixture repository, as built"

# Two of the tools procoder wants are resolved per-repository rather than
# from PATH — typescript-eslint is imported by an eslint config and ships
# no binary, and prettier's PHP plugin is looked up beside prettier. A
# rebuild wipes node_modules with everything else, so they are installed
# here or the fixture reports them missing forever. Needs network; a
# failure is reported and left, because a fixture that cannot reach npm is
# still a fixture worth checking.
if command -v npm >/dev/null 2>&1; then
	if npm install --no-audit --no-fund --silent -D typescript-eslint prettier @prettier/plugin-php >/dev/null 2>&1; then
		echo "fixture: repo-local lint tools installed" >&2
	else
		echo "fixture: npm install failed — typescript-eslint and the PHP plugin will report missing" >&2
	fi
fi

echo "$DEST"
