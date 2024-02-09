package common

type contextKey int

const StorageContextKey = contextKey(0)
const CurrentWorkspaceIdContextKey = contextKey(1)
const CurrentAuthorContextKey = contextKey(2)
const ListenerRefContextKey = contextKey(3)
