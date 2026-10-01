package audit

// RootHashForTest exports rootHash for black-box testing.
func RootHashForTest(entries []StoredEntry) string {
	return rootHash(entries)
}

// MerkleTreeRootForTest exports merkleTreeRoot for black-box testing.
func MerkleTreeRootForTest(nodes []string) string {
	return merkleTreeRoot(nodes)
}

// SignHashForTest exports signHash for black-box testing.
func SignHashForTest(key []byte, entryHash string) string {
	return signHash(key, entryHash)
}

// VerifySignatureForTest exports verifySignature for black-box testing.
func VerifySignatureForTest(key []byte, entryHash, signature string) bool {
	return verifySignature(key, entryHash, signature)
}
