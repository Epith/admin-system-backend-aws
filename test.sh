#!/bin/bash

# Set the base branch to develop
base_branch="main"

# Get the current branch
current_branch=$(git rev-parse --abbrev-ref HEAD)

# Checkout the base branch
git checkout $base_branch

# Get the list of changed folders
changed_folders=$(git diff-tree --name-only -r $base_branch..$current_branch -- functions | grep -v main.go)

# Build the changed folders
for folder in $changed_folders; do
  make build-$folder
done
