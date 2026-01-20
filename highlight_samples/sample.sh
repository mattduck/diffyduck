#!/bin/bash
# Sample bash script for syntax highlighting

set -euo pipefail

# Variables
NAME="world"
COUNT=42
ARRAY=(one two three)

# Function definition
greet() {
    local message="$1"
    echo "Hello, ${message}!"
}

# Conditionals
if [[ -n "$NAME" ]]; then
    greet "$NAME"
elif [[ $COUNT -gt 10 ]]; then
    echo "Count is large"
else
    echo "Default case"
fi

# Loops
for item in "${ARRAY[@]}"; do
    echo "Item: $item"
done

while read -r line; do
    echo "$line"
done < /etc/passwd

# Case statement
case "$NAME" in
    world)
        echo "Matched world"
        ;;
    *)
        echo "No match"
        ;;
esac

# Command substitution
TODAY=$(date +%Y-%m-%d)
FILES=$(ls -la)

# Here document
cat <<EOF
This is a heredoc.
Name: $NAME
Date: $TODAY
EOF

# Arithmetic
result=$((COUNT * 2 + 10))
echo "Result: $result"
