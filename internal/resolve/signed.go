package resolve

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

// checkSigned proves a checksum file with the project's signature (#5),
// when the catalog says the project signs it, and returns the text to read
// checksums from: for a clearsigned file only what the signature covers. A
// signature that fails is an error, so nothing is downloaded on the strength
// of a checksum file that may have been replaced.
func checkSigned(ctx context.Context, client *remote.Client, spec *catalog.Artifact, base, manifestURL *url.URL, vars map[string]string, body []byte, a *Artifact) ([]byte, error) {
	switch spec.Signature {
	case catalog.SignatureManifest:
		sig, sigURL, err := getSignature(ctx, client, spec, base, vars, a)
		if err != nil {
			return nil, err
		}
		if err := signer(spec).CheckDetached(bytes.NewReader(body), sig); err != nil {
			return nil, fmt.Errorf("the checksum file %s isn't signed by the project's key, so isoshelf won't trust it: %w", manifestURL, err)
		}
		a.Signed = sigURL
	case catalog.SignatureClearsigned:
		text, err := signer(spec).Clearsigned(body)
		if err != nil {
			return nil, fmt.Errorf("the checksum file %s isn't signed by the project's key, so isoshelf won't trust it: %w", manifestURL, err)
		}
		a.Signed = manifestURL.String()
		return text, nil
	}
	return body, nil
}

// fetchImageSignature fetches the signature over the image itself; update
// checks it against the downloaded bytes before placing them.
func fetchImageSignature(ctx context.Context, client *remote.Client, spec *catalog.Artifact, base *url.URL, vars map[string]string, a *Artifact) error {
	sig, sigURL, err := getSignature(ctx, client, spec, base, vars, a)
	if err != nil {
		return err
	}
	a.ImageSignature, a.Signed = sig, sigURL
	return nil
}

// getSignature fetches artifact.sig from the official site, as checksums are.
func getSignature(ctx context.Context, client *remote.Client, spec *catalog.Artifact, base *url.URL, vars map[string]string, a *Artifact) ([]byte, string, error) {
	if spec.Signer() == nil {
		return nil, "", fmt.Errorf("the catalog names no usable key for this signature")
	}
	if a.Filename == "" && strings.Contains(spec.Sig, "{file}") {
		return nil, "", fmt.Errorf("artifact.sig names the image, which isn't known until the checksum file is read")
	}
	withFile := map[string]string{"file": a.Filename}
	for k, v := range vars {
		withFile[k] = v
	}
	sigURL, err := expandURL(base, spec.Sig, withFile)
	if err != nil {
		return nil, "", fmt.Errorf("artifact.sig: %w", err)
	}
	resp, err := client.Get(ctx, sigURL.String())
	if err != nil {
		return nil, "", fmt.Errorf("couldn't fetch the project's signature %s: %w", sigURL, err)
	}
	if resp.URL.Host != sigURL.Host {
		return nil, "", fmt.Errorf("signature %s redirected to %s; signatures must come from the official site", sigURL, resp.URL.Host)
	}
	return resp.Body, sigURL.String(), nil
}

// signer is the entry's key; a nil one fails every check rather than
// passing it.
func signer(spec *catalog.Artifact) *verify.Keyring {
	if k := spec.Signer(); k != nil {
		return k
	}
	return &verify.Keyring{}
}
