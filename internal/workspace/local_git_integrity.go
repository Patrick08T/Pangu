package workspace

func localObjectCheckArgs(commit string) []string {
	// P4 metadata files are raw blobs, never interpreted as Git attributes/modules.
	// Ignore only reproduced metadata semantics; retain missing/corrupt/type checks.
	return []string{
		"-c", "fsck.gitmodulesName=ignore",
		"-c", "fsck.gitmodulesPath=ignore",
		"-c", "fsck.gitmodulesUrl=ignore",
		"-c", "fsck.gitmodulesUpdate=ignore",
		"-c", "fsck.gitattributesLarge=ignore",
		"-c", "fsck.gitattributesLineLength=ignore",
		"fsck", "--no-dangling", "--no-reflogs", commit,
	}
}
