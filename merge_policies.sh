#!/bin/bash

SOURCE_DIR1="your-local-path/visualizer/policies"
SOURCE_DIR2="your-local-path/flow/policies"
SOURCE_DIR3="your-local-path/cms/policies"
DEST_DIR="./policies"

rm -rf "$DEST_DIR"
mkdir -p "$DEST_DIR"

cp -r "$SOURCE_DIR1/." "$DEST_DIR/"
cp -r "$SOURCE_DIR2/." "$DEST_DIR/"
cp -r "$SOURCE_DIR3/." "$DEST_DIR/"
