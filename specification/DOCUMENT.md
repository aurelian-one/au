# Aurelian Workspace Specification

This file attempts to nail down a strict definition of the Aurelian Workspace document format, how it works, 
what structure it has, and how that structure should be interpreted by consumers.

Terms used below:

- `Workspace` - A collection of Todos and metadata. Clients collaborate on a Workspace level.
- `Todo` - A task within a Workspace.
- `Comment` - A attached Comment for a Todo

## 1. An Aurelian Workspace is a native uncompressed Automerge Document

An Aurelian Workspace is a raw byte array. These bytes follow the "Binary Document Format" as described in https://automerge.org/automerge-binary-format-spec/.

Since Automerge documents are amenable to compression, they may be compressed over the wire or stored compressed as
necessary, but this is not part of the specification here. Compression should be treated as opt-in by any interfaces
to allow for broader compatibility.

Aurelian workspaces are also not encrypted in any particular way in this specification. Any encryption is
left up to the client or server implementation.

Note that Aurelian Workspaces may be quite large since all history is stored and there may be embedded binary attachments.
However, because documents are usually synchronised by exchanging only the changes required, there is usually relatively
little bandwidth penalty as the document grows.

## 2. The structure

### 2.1 Top level fields

#### `alias` - KindStr

Alias is a human-readable alias string for the workspace. The workspace is usually identified by some external UUID which
is not stored in the document, so this alias provides a common and shared understanding of what kinds of tasks and todos
the workspace contains and is consistent across all consumers of the workspace.

Examples might be:

- `Daily Tasks`
- `Apollo Project`
- `Default`

The contents should be valid single-line UTF-8 according to section 3.1 in this document and should contain at most 100 "characters".

#### `created_at` - KindTime

The created at timestamp of the workspace is stored along with the alias to support conflict detection between workspaces
that may share an alias and to act as a stable sorting criteria for clients. This is stored as a UTC timestamp. This
may be defaulted to the unix epoch if it is missing.

#### `todos` - KindMap

This is the core map of Todo Id's to Todo contents. Each key in this map should be a valid ULID, see https://github.com/ulid/spec. 
This Id is assumed to be unique within the workspace, but not unique across workspaces.

The value of the entry is a Todo (see 2.2).

### 2.2 The Todo

Each entry in the top-level `todos` map is itself an Automerge Map structure. It cotains:

#### `title` - KindText

The title of the Todo. This describes the goal or definition-of-done of the Todo and is therefore a static string that
doesn't support splicing and merging. It is strictly LLW.

The contents should be valid single-line UTF-8 according to section 3.1 in this document and should contain at most 200 "characters".

#### `description` - KindText

The description is the longer form multiline content of the task. This is optional and evolving content which indicates
the current understanding of the status and what it takes to complete the task. Changes may be contributed concurrently 
by multiple participants.

The contents should be valid multi-line UTF-8 according to section 3.1 in this document and should contain at most 5000 "characters".

#### `created_at` - KindTime

The time the Todo was created. Usually not editable.

#### `status` - KindStr

The required status of the todo. The status is either `open` or `closed`. All todos default to `open`. `closed` should be
used to indicate that the Todo is complete but still viewable and searchable in the client. There is no restriction
preventing todos from being set to `open` from `closed`.

#### `annotations` - KindMap

Annotations are used to support custom applications built on top of the Aurelian format. The annotation key should be a 
uri that SHOULD point to a reference describing the annotation meaning.

Setting an annotation to an empty string should be equivalent to removing the annotation. Clients should enforce 
validation on adding or modifying annotations they understand and treat any other annotations as read-only.

Examples of how annotations may be used:

- Storing a human readable status reason along with the todo (eg: `https://aurelian.one/annotations/status-open-reason: In Progress`)
- Storing a machine-readable todo Rank (eg: `https://aurelian.one/annotations/rank: 0`) to allow basic prioritisation of tasks
- Storing a url-encoded Label (eg: `https://aurelian.one/annotations/label#My%20Label: true`) to allow grouping and filtering of tasks
- Hiding a todo until a target date/time (eg: `https//github.com/my-au-bot/hide-until: 2025-01-01`)

The `aurelian` and `aurelian.one` uri hosts are reserved for use by the Aurelian project at this time.

#### `comments` - KindMap

Comments are used to add supporting (usually immutable) attachment content to each Todo. This will usually be Markdown
content like the Todo description itself but can be extended with other content types in the future. This is why there
is an associated media type.

Each key in this map should be a valid ULID, see https://github.com/ulid/spec. This Id is assumed to be unique within the 
todo, but not unique across todos.

Each comment has the format:

#### `created_at` - KindTime

The time the Comment was created. Usually not editable.

#### `media_type` - KindStr

The media/content type of the Comment. This should be a valid MIME Type https://datatracker.ietf.org/doc/html/rfc2046 but
will usually default to `text/markdown`.

#### `content` - KindBytes

The content of the comment. This is assumed to be UTF-8 encoded for `text/markdown`. 

## 3. Appendix

### 3.1 Supported unicode

Edits to any unicode fields should only be accepted if they comply with the following:

- No illegal byte sequences or non shortest-form characters.
- All content should be normalized using NFC.
- Only allow printable characters from categories `L, M, N, P, S` and ascii space. Allow ascii newline, carriage return,
    and tab if the field is multiline.
