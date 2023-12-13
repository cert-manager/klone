#!/bin/sh

wget -O checksums.txt https://github.com/cert-manager/klone/releases/download/latest/checksums.txt
wget -O checksums.txt.cosign.bundle https://github.com/cert-manager/klone/releases/download/latest/checksums.txt.cosign.bundle

cosign verify-blob checksums.txt \
    --bundle checksums.txt.cosign.bundle \
    --certificate-identity=https://github.com/cert-manager/klone/.github/workflows/release.yml@refs/tags/v0.0.1-alpha.0 \
    --certificate-oidc-issuer=https://token.actions.githubusercontent.com
