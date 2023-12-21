#!/bin/bash

# Function to print messages in green
print_green() {
    echo -e "\033[0;32m$1\033[0m"
}

# Function to print messages in red
print_red() {
    echo -e "\033[0;31m$1\033[0m"
}

# Check if Golang is installed
if go env > /dev/null 2>&1; then
    print_green "Golang is installed"
else
    print_red "Golang is not installed"
fi

# Check if $GOPATH is in PATH
if echo $PATH | grep -q "$(which go)"; then
    print_green "\$GOPATH is in PATH"
else
    print_red "\$GOPATH is not in PATH"
fi

# Extend this script with more checks as needed

