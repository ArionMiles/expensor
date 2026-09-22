#!/bin/sh
set -eu

version=${EXPENSOR_VERSION:-${1:-}}
if [ -z "$version" ]; then
  echo "usage: EXPENSOR_VERSION=<version> install.sh" >&2
  exit 2
fi

os=${EXPENSOR_OS:-$(uname -s)}
case $(printf '%s' "$os" | tr '[:upper:]' '[:lower:]') in
  linux) os=linux ;;
  darwin) os=darwin ;;
  *) echo "unsupported operating system: $os" >&2; exit 2 ;;
esac

arch=${EXPENSOR_ARCH:-$(uname -m)}
case "$arch" in
  amd64|x86_64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported architecture: $arch" >&2; exit 2 ;;
esac

download_root=${EXPENSOR_DOWNLOAD_ROOT:-"https://github.com/ArionMiles/expensor/releases/download/$version"}
install_dir=${EXPENSOR_INSTALL_DIR:-"$HOME/.local/bin"}
config_dir=${EXPENSOR_CONFIG_DIR:-"${XDG_CONFIG_HOME:-$HOME/.config}/expensor"}
archive="expensor_${version}_${os}_${arch}.tar.gz"
manifest="expensor_${version}_checksums.txt"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

fetch() {
  source=$1
  destination=$2
  case "$download_root" in
    http://*|https://*)
      if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$source" -o "$destination"
      elif command -v wget >/dev/null 2>&1; then
        wget -qO "$destination" "$source"
      else
        echo "curl or wget is required" >&2
        return 1
      fi
      ;;
    *) cp "$source" "$destination" ;;
  esac
}

fetch "$download_root/$manifest" "$tmp/$manifest"
fetch "$download_root/$archive" "$tmp/$archive"
awk -v archive="$archive" '$2 == archive { print }' "$tmp/$manifest" >"$tmp/checksum"
if [ "$(wc -l <"$tmp/checksum" | tr -d ' ')" -ne 1 ]; then
  echo "checksum entry not found for $archive" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmp" && sha256sum -c checksum)
elif command -v shasum >/dev/null 2>&1; then
  (cd "$tmp" && shasum -a 256 -c checksum)
else
  echo "sha256sum or shasum is required" >&2
  exit 1
fi

tar -tzf "$tmp/$archive" | LC_ALL=C sort >"$tmp/archive-files"
printf '%s\n' LICENSE NOTICE config.toml.example expensor >"$tmp/expected-files"
if ! diff -u "$tmp/expected-files" "$tmp/archive-files" >/dev/null; then
  echo "release archive has an unexpected layout" >&2
  exit 1
fi
mkdir "$tmp/extract"
tar -xzf "$tmp/$archive" -C "$tmp/extract"

key_file="$config_dir/secret.key"
new_key=
if [ ! -e "$key_file" ]; then
  openssl_command=${EXPENSOR_OPENSSL:-openssl}
  if ! command -v "$openssl_command" >/dev/null 2>&1; then
    echo "OpenSSL is required to create the encryption key" >&2
    exit 1
  fi
  new_key="$tmp/secret.key"
  "$openssl_command" rand -base64 32 >"$new_key"
  chmod 0600 "$new_key"
fi

config_file="$config_dir/config.toml"
new_config=
if [ ! -e "$config_file" ]; then
  new_config="$tmp/config.toml"
  escaped_key=$(printf '%s' "$key_file" | sed 's/[\&|]/\\&/g')
  sed "s|@SECRET_KEY_FILE@|$escaped_key|g" "$tmp/extract/config.toml.example" >"$new_config"
  chmod 0600 "$new_config"
fi

mkdir -p "$install_dir" "$config_dir"
chmod 0700 "$config_dir"
if [ -n "$new_key" ] && [ ! -e "$key_file" ]; then
  key_tmp="$config_dir/.secret.key.$$"
  cp "$new_key" "$key_tmp"
  chmod 0600 "$key_tmp"
  mv -f "$key_tmp" "$key_file"
fi
if [ -n "$new_config" ] && [ ! -e "$config_file" ]; then
  config_tmp="$config_dir/.config.toml.$$"
  cp "$new_config" "$config_tmp"
  chmod 0600 "$config_tmp"
  mv -f "$config_tmp" "$config_file"
fi

binary_tmp="$install_dir/.expensor.$$"
cp "$tmp/extract/expensor" "$binary_tmp"
chmod 0755 "$binary_tmp"
mv -f "$binary_tmp" "$install_dir/expensor"

printf 'Installed Expensor to %s\n' "$install_dir/expensor"
printf 'Configuration: %s\n' "$config_file"
