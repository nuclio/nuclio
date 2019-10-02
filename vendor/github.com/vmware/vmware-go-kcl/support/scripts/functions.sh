set -ex

# PROJ_ROOT specifies the project root
export PROJ_ROOT="$HMAKE_PROJECT_DIR"

# Add /go in GOPATH because that's the original GOPATH in toolchain
export GOPATH=/go:$PROJ_ROOT

local_go_pkgs() {
    find './clientlibrary/' -name '*.go' | \
        grep -Fv '/vendor/' | \
        grep -Fv '/go/' | \
        grep -Fv '/gen/' | \
        grep -Fv '/tmp/' | \
        grep -Fv '/run/' | \
        grep -Fv '/tests/' | \
        sed -r 's|(.+)/[^/]+\.go$|\1|g' | \
        sort -u
}

version_suffix() {
    local suffix=$(git log -1 --format=%h 2>/dev/null || true)
    if [ -n "$suffix" ]; then
        test -z "$(git status --porcelain 2>/dev/null || true)" || suffix="${suffix}+"
        echo -n "-g${suffix}"
    else
        echo -n -dev
    fi
}

git_commit_hash() {
	echo $(git rev-parse --short HEAD)
}

# Due to Go plugin genhash algorithm simply takes full source path
# from archive, it generates different plugin hash if source path of
# shared pkg is different, and causes load failure.
# as a workaround, lookup shared pkg and place it to fixed path
FIX_GOPATH=/tmp/go

fix_go_pkg() {
    local pkg="$1" base
    for p in ${GOPATH//:/ }; do
        if [ -d "$p/src/$pkg" ]; then
            base="$p"
            break
        fi
    done

    if [ -z "$base" ]; then
        echo "Package $pkg not found in GOPATH: $GOPATH" >&2
        return 1
    fi

    local fix_pkg_path="$FIX_GOPATH/src/$pkg"
    rm -f "$fix_pkg_path"
    mkdir -p "$(dirname $fix_pkg_path)"
    ln -s "$base/src/$pkg" "$fix_pkg_path"
}
