#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR/../../"

in_dir=${1:-${ROOT_DIR}web/content/in/}
out_dir="$in_dir/../out/"

for file in "$in_dir"*.md; do
	if [[ -f "$file" ]]; then
		output_file="${out_dir}$(basename $file .md).css"

		pandoc --highlight-style="$SCRIPT_DIR/monokai-tasty.theme" --template="$SCRIPT_DIR/template-css.html" "$file" -o "$output_file"
		npx prettier --write $output_file

		echo "Converted $file to $output_file"
	fi
done
