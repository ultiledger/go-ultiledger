#!/bin/sh

set -e

if [ ! -f "build/env.sh" ]; then
    echo "$0 must be run from the root of the repository."
    exit 2
fi

# Create fake Go workspace if it doesn't exist yet.
workspace="$PWD/build/_workspace"
root="$PWD"
ultdir="$workspace/src/github.com/ultiledger"
if [ ! -L "$ultdir/go-ultiledger" ]; then
    mkdir -p "$ultdir"
    cd "$ultdir"
    ln -s ../../../../../. go-ultiledger
    cd "$root"
fi

# Set up the environment to use the workspace.
GOPATH="$workspace"
export GOPATH

GOBIN="$PWD/build/bin"
export GOBIN

# Run the command inside the workspace.
cd "$ultdir/go-ultiledger/cmd/ult"

# Launch the arguments with the configured environment.
exec "$@"
