#!/bin/sh

# Regex pattern to validate Conventional Commit message
CONVENTIONAL_COMMIT_REGEX="^(feat|fix|docs|style|refactor|perf|test|chore|ci|build)(\(\S+\))?!?:\s.+"

# Read the commit message
COMMIT_MSG=$(cat "$1")

if ! echo "$COMMIT_MSG" | grep -Eq "$CONVENTIONAL_COMMIT_REGEX"; then
    echo "ERROR: Commit message does not follow Conventional Commits format."
	echo "Formats: feat|fix|docs|style|refactor|perf|test|chore|ci|build"
    echo "Refer: https://www.conventionalcommits.org/"
    exit 1
fi

