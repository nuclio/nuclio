#!/bin/bash

set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

if [ $# == 0 ]; then
	echo "Usage: $0 PKG"
	echo
	echo -e "Valid packages:\n\t$(ls -d */ | sed 's/\/*$//g' | tr '\n' ' ')"
	exit 1
fi

# Check that the fuzzing tools are available.
for TOOL in go-fuzz go-fuzz-build; do
	command -v $TOOL >/dev/null 2>&1 || {
		echo "Aborting: could not locate $TOOL."; exit 1;
	}
done

# Clone the initial work directory if it does not exist.
if [ ! -d ".work" ]; then
	echo "Fuzzing workdir does not exist."
	git clone https://github.com/dsnet/compress-fuzz.git .work
fi

PKG=$(echo $1 | sed 's/\/*$//g')
PKG_PATH="github.com/dsnet/compress/internal/tool/fuzz"
shift

echo "Building..."
go-fuzz-build -o=".work/$PKG-fuzz.zip" $PKG_PATH/$PKG

echo "Fuzzing..."
go-fuzz -bin=".work/$PKG-fuzz.zip" -workdir=".work/$PKG" "$@"
