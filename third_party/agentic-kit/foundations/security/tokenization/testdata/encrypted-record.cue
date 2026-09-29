package tokenization

encryptedRecord: #EncryptedRecord & {
	Version: 1
	Token: {
		Value:     "tok_pii_0000000000000000000000000000000000000000000000000000000000000000"
		Class:     "pii"
		Namespace: "fixture"
	}
	// Shape fixture only: real authentication is covered by Go tests.
	Ciphertext: '0000000000000000000000000000'
}
