# No silent green: every gate says when it did not run

Status: active
Created: 2026-08-21

## Goal

A green gate means the code was checked. Today it can also mean the
machine was empty: a missing linter reports as info, a formatter without a
project config reports the file out of scope, and both let `procoder
check` exit 0 over code nothing read.

This sprint closes every route to that verdict — in every domain, not the
two that happened to be noticed — ships a working default wherever the
project brought none, and leaves an audit over the source so a domain
written next month cannot reopen it.

What it must not do is turn the gate into noise: a file type procoder does
not claim stays out of scope, stays silent, stays green.

## Stories

<!-- pulled below -->

## Retro

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->
