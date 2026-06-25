#!/bin/sh -e

# shellcheck disable=SC2002
version="$(cat plugin.yaml | grep "version" | cut -d ' ' -f 2)"
os=$(uname)
arch=$(uname -m)

case "${arch}" in
    x86_64|amd64)
        arch="amd64"
        ;;
    arm64|aarch64)
        arch="arm64"
        ;;
    *)
        echo "Unsupported architecture: ${arch}" >&2
        exit 1
        ;;
esac

echo "Downloading and installing spray v${version} for ${os}/${arch}..."

url=""
if [ "${os}" = "Linux" ] ; then
    url="https://github.com/ThalesGroup/helm-spray/releases/download/v${version}/helm-spray-linux-${arch}.tar.gz"
elif [ "${os}" = "Darwin" ] ; then
    url="https://github.com/ThalesGroup/helm-spray/releases/download/v${version}/helm-spray-darwin-${arch}.tar.gz"
else
    url="https://github.com/ThalesGroup/helm-spray/releases/download/v${version}/helm-spray-windows-amd64.tar.gz"
fi

mkdir -p "bin"
mkdir -p "releases/v${version}"

# Download with curl if possible.
# shellcheck disable=SC2230
if [ -x "$(which curl 2>/dev/null)" ]; then
    curl -sSL "${url}" -o "releases/v${version}.tar.gz"
else
    wget -q "${url}" -O "releases/v${version}.tar.gz"
fi
tar xzf "releases/v${version}.tar.gz" -C "releases/v${version}"
if [ "${os}" = "Linux" ] || [ "${os}" = "Darwin" ] ; then
    mv "releases/v${version}/bin/helm-spray" "bin/helm-spray"
else
    mv "releases/v${version}/bin/helm-spray.exe" "bin/helm-spray.exe"
fi
mv "releases/v${version}/plugin.yaml" .
mv "releases/v${version}/README.md" .
mv "releases/v${version}/LICENSE" .
