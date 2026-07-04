#!/bin/sh
# Integration checks for yup-tee, run inside a Debian (GNU coreutils) container.
#
# tee copies stdin to stdout AND to each FILE operand. Every check feeds the
# same input to yup-tee and GNU `tee` (writing to disjoint /tmp paths) and
# asserts BOTH the captured stdout AND the resulting file contents are
# byte-identical between the two.
#
# parity LABEL ARGS...  — split into the yup-* and gnu-* file paths internally.
set -eu

fails=0
sample='alpha
beta

gamma'

# parity LABEL ARGS... where every "@N" in ARGS is the Nth file operand; the
# helper substitutes /tmp/yup<N> for yup-tee and /tmp/gnu<N> for GNU tee, runs
# both on the same stdin, then compares stdout and each file pairwise.
parity() {
  label=$1
  shift
  ours_args=$(echo "$@" | sed 's#@\([0-9]\)#/tmp/yup\1#g')
  gnu_args=$(echo "$@" | sed 's#@\([0-9]\)#/tmp/gnu\1#g')

  # shellcheck disable=SC2086
  ours_out=$(printf '%s\n' "$sample" | yup-tee $ours_args 2>/dev/null || true)
  # shellcheck disable=SC2086
  gnu_out=$(printf '%s\n' "$sample" | tee $gnu_args 2>/dev/null || true)

  ok=1
  if [ "$ours_out" != "$gnu_out" ]; then
    ok=0
    printf 'FAIL  parity  tee %s (stdout)\n        gnu:  %s\n        ours: %s\n' "$label" "$gnu_out" "$ours_out"
  fi
  # Compare every file operand that appears in the args.
  for n in 1 2; do
    case " $* " in
      *"@$n"*)
        if ! cmp -s "/tmp/yup$n" "/tmp/gnu$n"; then
          ok=0
          printf 'FAIL  parity  tee %s (file @%s)\n        gnu:  %s\n        ours: %s\n' \
            "$label" "$n" "$(cat /tmp/gnu$n)" "$(cat /tmp/yup$n)"
        fi
        ;;
    esac
  done

  if [ "$ok" -eq 1 ]; then
    printf 'ok    parity  tee %s\n' "$label"
  else
    fails=$((fails + 1))
  fi
}

# Passthrough only: no file operands, stdout must match.
parity "(passthrough)"

# Single file: stdout AND the file must match GNU.
parity "@1" @1

# Two files: both files plus stdout must match GNU.
parity "@1 @2" @1 @2

# Append mode (-a): seed both files identically, then append and compare.
printf 'preexisting\n' >/tmp/yup1
printf 'preexisting\n' >/tmp/gnu1
parity "-a @1" -a @1

if [ "$fails" -ne 0 ]; then
  printf '\n%s check(s) failed\n' "$fails"
  exit 1
fi
printf '\nall checks passed\n'
