# Aurelian CLI Config Directory convention

This document describes the layout on disk of the Aurelian config directory used by CLI tooling.

This is less of a specification and more of a convention to allow co-operation between different CLI tools on the same machine.

## Location

By default, this is `~/.au` but can be overriden by an `AU_DIRECTORY` environment variable or any appropriate flag on the CLI implementation.

## The current workspace

The current workspace that will be used by CLI tools if no override is provided is stored at `${AU_DIRECTORY}/current`. This file contains just the id of the current workspace.

This is usually overriden by the `AU_WORKSPACE` environment variable or any appropriate flag on the CLI implementation.

## Workspace files

Each Workspace file is stored as `${AU_DIRECTORY}/<ID>.automerge`. This file is the native uncompressed form of the [Aurelian Document](./DOCUMENT.md). We _may_ support gzip compressed forms as `.automerge.gzip`.

The current author setting for the document is stored at `${AU_DIRECTORY}/<ID>.author`. But this can be overriden by `AU_AUTHOR` environment variable or any appopriate flag on the CLI implementation.

A lock file may exist at `${AU_DIRECTORY}/<ID>.lock`. This is used for file-based locking to ensure CLI tools are not concurrently attempting to modify this file.
