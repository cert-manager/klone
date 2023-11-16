package git

import (
	"bytes"
	"fmt"
	"os"
)

func GetHash(repoURL string, ref string) (string, error) {
	outBuffer := &bytes.Buffer{}
	if err := runGitCmd(".", outBuffer, os.Stderr, "ls-remote", repoURL, ref); err != nil {
		return "", err
	}

	outFields := bytes.Fields(outBuffer.Bytes())
	if len(outFields) != 2 {
		return "", fmt.Errorf("could not find %s@%s", repoURL, ref)
	}

	hash := outFields[0]

	return string(hash), nil
}
