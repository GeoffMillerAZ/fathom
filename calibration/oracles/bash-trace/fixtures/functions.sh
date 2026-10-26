#!/usr/bin/env bash
shopt -s expand_aliases
target_one() { :; }
target_two() { :; }
alias shorthand=target_one
driver() {
  shorthand
}
driver
