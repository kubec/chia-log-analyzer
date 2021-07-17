#!/usr/bin/env bash

GIT_COMMIT=$(git rev-list -1 HEAD)
echo 'Building GIT version:' $GIT_COMMIT
package=chia-log-analyzer.go
#package=$1
#if [[ -z "$package" ]]; then
#  echo "usage: $0 <package-name>"
#  exit 1
#fi
package_split=(${package//\// })
package_name=${package_split[-1]}

platforms=("windows/amd64" "linux/amd64" "linux/arm" "darwin/amd64")

for platform in "${platforms[@]}"
do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}
    output_name=$package_name'-'$GOOS'-'$GOARCH
    if [ $GOOS = "windows" ]; then
        output_name+='.exe'
    fi
    echo 'Building OS/ARCH: '$GOOS'/'$GOARCH
    env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "-X main.GitCommit=$GIT_COMMIT" -o ./builds/$output_name $package
    if [ $? -ne 0 ]; then
        echo 'An error has occurred! Aborting the script execution...'
        exit 1
    fi
done
echo "Building done"