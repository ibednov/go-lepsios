package crypto

func BackupCodesGenerate() ([]string, error) {
	codes := make([]string, 10)
	for i := 0; i < 10; i++ {
		code, err := GenerateRefreshToken()
		if err != nil {
			return nil, err
		}
		if len(code) > 8 {
			codes[i] = code[:8]
		} else {
			codes[i] = code
		}
	}
	return codes, nil
}

type BackupCodesGenerateAndHashInput struct {
	Secret        string
	EncryptionKey string
}

type BackupCodesGenerateAndHashOutput struct {
	EncryptedSecret string
	Codes           []string
	HashedCodes     []string
	Error           error
}

func BackupCodesGenerateAndHash(input BackupCodesGenerateAndHashInput) BackupCodesGenerateAndHashOutput {
	var codes []string
	var err error

	codes, err = BackupCodesGenerate()
	if err != nil {
		return BackupCodesGenerateAndHashOutput{Error: err}
	}

	secret, err := EncryptAES256GCM([]byte(input.Secret), []byte(input.EncryptionKey))
	if err != nil {
		return BackupCodesGenerateAndHashOutput{Error: err}
	}

	hashedCodes := make([]string, len(codes))
	for i, code := range codes {
		hashedCodes[i] = HashToken(code)
	}

	return BackupCodesGenerateAndHashOutput{
		EncryptedSecret: secret,
		Codes:           codes,
		HashedCodes:     hashedCodes,
	}
}
