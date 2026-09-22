#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

mkdir -p "$tmp/input" "$tmp/releases"
for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  printf '#!/bin/sh\nprintf "expensor %s\\n"\n' "$target" >"$tmp/input/expensor_$target"
  chmod 0755 "$tmp/input/expensor_$target"
done
go run "$root/scripts/release/archive.go" \
  -input "$tmp/input" -output "$tmp/releases" -version 1.2.3 \
  -config "$root/deploy/config.toml.example" -license "$root/LICENSE" -notice "$root/NOTICE"

mode() {
  if [ "$(uname -s)" = Darwin ]; then
    /usr/bin/stat -f '%Lp' "$1"
  else
    stat -c '%a' "$1"
  fi
}

install_dir="$tmp/home/.local/bin"
config_dir="$tmp/home/.config/expensor"
EXPENSOR_VERSION=1.2.3 \
EXPENSOR_DOWNLOAD_ROOT="$tmp/releases" \
EXPENSOR_INSTALL_DIR="$install_dir" \
EXPENSOR_CONFIG_DIR="$config_dir" \
EXPENSOR_OS=linux EXPENSOR_ARCH=amd64 \
  "$root/install.sh"

test -x "$install_dir/expensor"
test "$(mode "$config_dir")" = 700
test "$(mode "$config_dir/config.toml")" = 600
test "$(mode "$config_dir/secret.key")" = 600
grep -q "secret_key_file = \"$config_dir/secret.key\"" "$config_dir/config.toml"

printf 'preserved config\n' >"$config_dir/config.toml"
printf 'preserved key\n' >"$config_dir/secret.key"
EXPENSOR_VERSION=1.2.3 \
EXPENSOR_DOWNLOAD_ROOT="$tmp/releases" \
EXPENSOR_INSTALL_DIR="$install_dir" \
EXPENSOR_CONFIG_DIR="$config_dir" \
EXPENSOR_OS=linux EXPENSOR_ARCH=amd64 \
  "$root/install.sh"
test "$(cat "$config_dir/config.toml")" = "preserved config"
test "$(cat "$config_dir/secret.key")" = "preserved key"

mkdir -p "$tmp/rollback/bin"
printf 'existing binary\n' >"$tmp/rollback/bin/expensor"
if EXPENSOR_VERSION=1.2.3 \
  EXPENSOR_DOWNLOAD_ROOT="$tmp/releases" \
  EXPENSOR_INSTALL_DIR="$tmp/rollback/bin" \
  EXPENSOR_CONFIG_DIR="$tmp/rollback/config" \
  EXPENSOR_OPENSSL="$tmp/missing-openssl" \
  EXPENSOR_OS=linux EXPENSOR_ARCH=amd64 \
    "$root/install.sh"; then
  echo "installer accepted a missing OpenSSL command" >&2
  exit 1
fi
test "$(cat "$tmp/rollback/bin/expensor")" = "existing binary"
test ! -e "$tmp/rollback/config/config.toml"

cp "$tmp/releases/expensor_1.2.3_linux_amd64.tar.gz" "$tmp/corrupt.tar.gz"
printf 'corrupt\n' >>"$tmp/corrupt.tar.gz"
cp "$tmp/corrupt.tar.gz" "$tmp/releases/expensor_1.2.3_linux_amd64.tar.gz"
if EXPENSOR_VERSION=1.2.3 \
  EXPENSOR_DOWNLOAD_ROOT="$tmp/releases" \
  EXPENSOR_INSTALL_DIR="$tmp/rejected/bin" \
  EXPENSOR_CONFIG_DIR="$tmp/rejected/config" \
  EXPENSOR_OS=linux EXPENSOR_ARCH=amd64 \
    "$root/install.sh"; then
  echo "installer accepted a corrupt archive" >&2
  exit 1
fi
test ! -e "$tmp/rejected/bin/expensor"
