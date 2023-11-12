#!/bin/bash

# Set the base branch
base_branch="develop"

# Get the current branch
current_branch=$(git rev-parse --abbrev-ref HEAD)

# Get the list of changed folders
changed_folders=$(git diff-tree --name-only -r $base_branch..$current_branch)

# Build the changed folders
for folder in $changed_folders; do
  if [[ -d "$folder" ]]; then
    make build-$folder
  fi
done
