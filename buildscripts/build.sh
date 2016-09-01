#!/bin/bash

_init() {
    # Save release LDFLAGS
    LDFLAGS=$(go run buildscripts/gen-ldflags.go)

    # Extract release tag
    release_tag=$(echo $LDFLAGS | awk {'print $4'} | cut -f2 -d=)

    # Verify release tag.
    if [ -z "$release_tag" ]; then
        echo "Release tag cannot be empty. Please check return value of \`go run buildscripts/gen-ldflags.go\`"
        exit 1;
    fi

    # Extract release string.
    release_str=$(echo $MINIO_RELEASE | tr '[:upper:]' '[:lower:]')

    # Verify release string.
    if [ -z "$release_str" ]; then
        echo "Release string cannot be empty. Please set \`MINIO_RELEASE\` env variable."
        exit 1;
    fi

    # List of supported architectures
    SUPPORTED_OSARCH='linux/386 linux/amd64 linux/arm windows/386 windows/amd64 darwin/amd64 freebsd/amd64'

    ## System binaries
    CP=`which cp`
    SHASUM=`which shasum`
    SED=`which sed`
}

go_build() {
    local osarch=$1
    os=$(echo $osarch | cut -f1 -d'/')
    arch=$(echo $osarch | cut -f2 -d'/')
    package=$(go list -f '{{.ImportPath}}')
    echo -n "-->"
    printf "%15s:%s\n" "${osarch}" "${package}"

    # Release binary name
    release_bin="$release_str/$os-$arch/$(basename $package).$release_tag"
    # Release binary downloadable name
    release_real_bin="$release_str/$os-$arch/$(basename $package)"
    # Release shasum name
    release_shasum="$release_str/$os-$arch/$(basename $package).shasum"

    # Go build to build the binary.
    GOOS=$os GOARCH=$arch go build --ldflags "${LDFLAGS}" -o $release_bin

    # Create copy
    if [ $os == "windows" ]; then
        $CP -p $release_bin ${release_real_bin}.exe
    else
        $CP -p $release_bin $release_real_bin
    fi

    # Calculate shasum
    shasum_str=$(${SHASUM} ${release_bin})
    echo ${shasum_str} | $SED "s/$release_str\/$os-$arch\///g" > $release_shasum
}

main() {
    # Build releases.
    echo "Executing $release_str builds for OS: ${SUPPORTED_OSARCH}"
    echo  "Choose an OS Arch from the below"
    for osarch in ${SUPPORTED_OSARCH}; do
        echo ${osarch}
    done

    read -p "If you want to build for all, Just press Enter: " chosen_osarch
    if [ "$chosen_osarch" = "" ]; then
        for each_osarch in ${SUPPORTED_OSARCH}; do
            go_build ${each_osarch}
        done
    else
        go_build ${chosen_osarch}
    fi

}

# Run main.
_init && main
