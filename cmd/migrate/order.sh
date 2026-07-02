#!/bin/bash

if [ $# -ne 2 ]; then
  echo "Usage: $0 <table_name> <new_number>"
  exit 1
fi

TABLE_NAME=$1
NEW_NUMBER=$2

MIGRATIONS_DIR="migrations/postgres"

if [ ! -d "$MIGRATIONS_DIR" ]; then
  echo "Error: Directory $MIGRATIONS_DIR does not exist."
  exit 1
fi

NEW_FORMATTED=$(printf "%06d" $NEW_NUMBER)

COUNT=0

for file in "$MIGRATIONS_DIR"/*_*"$TABLE_NAME"*.sql; do
  if [ -f "$file" ]; then
    filename=$(basename "$file")

    current_prefix=$(echo "$filename" | grep -o "^[0-9]\+")

    new_filename="${filename/$current_prefix/$NEW_FORMATTED}"

    mv "$file" "$MIGRATIONS_DIR/$new_filename"
    echo "Renamed: $filename -> $new_filename"
    COUNT=$((COUNT + 1))
  fi
done

if [ $COUNT -eq 0 ]; then
  echo "No files containing '$TABLE_NAME' were found in $MIGRATIONS_DIR."
else
  echo "Renamed $COUNT file(s)."
fi