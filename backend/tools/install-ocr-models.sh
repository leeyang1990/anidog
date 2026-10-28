#!/bin/sh
# Same Apache-2.0 upstream models for production and contract tests. Keep the
# verified distribution file when identical; never accept a partial download.
set -eu
ocr_target=${1:?destination directory required}
ocr_source=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
mkdir -p "$ocr_target"
ocr_temp=
trap 'if [ -n "$ocr_temp" ]; then rm -f "$ocr_temp"; fi' EXIT HUP INT TERM
while read -r ocr_language ocr_sha ocr_url; do
    ocr_file="$ocr_target/$ocr_language.traineddata"
    if [ -f "$ocr_file" ] && printf '%s  %s\n' "$ocr_sha" "$ocr_file" | sha256sum -c - >/dev/null 2>&1; then
        continue
    fi
    ocr_temp=$(mktemp "$ocr_target/.$ocr_language.XXXXXX")
    ocr_attempt=0
    while :; do
        ocr_attempt=$((ocr_attempt + 1))
        if timeout 45 wget -q -T 30 -O "$ocr_temp" "$ocr_url" && printf '%s  %s\n' "$ocr_sha" "$ocr_temp" | sha256sum -c - >/dev/null 2>&1; then
            break
        fi
        if [ "$ocr_attempt" -ge 3 ]; then
            printf 'Cannot obtain verified OCR model: %s\n' "$ocr_language" >&2
            exit 1
        fi
    done
    chmod 644 "$ocr_temp"
    mv "$ocr_temp" "$ocr_file"
    ocr_temp=
done < "$ocr_source/ocr-models.tsv"
