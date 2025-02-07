package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
)

func GetHash(ctx context.Context, repoURL string, ref string) (string, error) {
	outBuffer := &bytes.Buffer{}
	if err := runGitCmd(ctx, ".", outBuffer, os.Stderr, "ls-remote", repoURL, ref); err != nil {
		return "", err
	}

	outFields := bytes.Fields(outBuffer.Bytes())
	if len(outFields) != 2 {
		return "", fmt.Errorf("could not find %s@%s", repoURL, ref)
	}

	hash := outFields[0]

	return string(hash), nil
}
