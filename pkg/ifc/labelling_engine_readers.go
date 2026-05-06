package ifc

// LabelGetMe returns a label for get_me: trusted, universal readers.
func LabelGetMe() ReadersSecurityLabel {
	return PublicTrusted()
}
