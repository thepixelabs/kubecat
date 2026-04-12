#!/bin/bash

# Default target
TARGET="build/appicon.png"

# check if magick is installed
if ! command -v magick &> /dev/null; then
    echo "Error: ImageMagick (magick) is not installed."
    exit 1
fi

# Use input file if provided
if [ "$1" ]; then
    echo "Copying $1 to $TARGET..."
    cp "$1" "$TARGET"
fi

if [ ! -f "$TARGET" ]; then
    echo "Error: $TARGET not found."
    exit 1
fi

echo "Resizing and optimizing $TARGET..."
# Resize to 1024x1024 (Max macOS icon size) and optimize
magick "$TARGET" -resize 1024x1024\> \
    -alpha set -background none \
    \( +clone -alpha transparent -fill white -draw "roundrectangle 0,0 %[fx:w-1],%[fx:h-1] %[fx:w*0.223],%[fx:h*0.223]" \) \
    -compose DstIn -composite \
    -strip \
    -colors 256 \
    -define png:compression-level=9 \
    -define png:compression-filter=5 \
    -define png:compression-strategy=1 \
    "$TARGET"

echo "Done. Icon updated at $TARGET."
echo "New size: $(du -h "$TARGET" | cut -f1)"
echo "Remember to run 'wails build' to apply changes."
