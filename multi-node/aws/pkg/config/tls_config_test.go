package config

import (
	"bytes"
	"io"
	"testing"

	"compress/gzip"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/blobutil"
)

func genTLSConfig(t *testing.T) *TLSConfig {
	config, err := newConfigFromBytes([]byte(MinimalConfigYaml))
	if err != nil {
		t.Fatalf("failed generating config: %v", err)
	}
	tlsConfig := newTLSConfig()

	if err := tlsConfig.generateAllTLS(config); err != nil {
		t.Fatalf("failed generating tls: %v", err)
	}

	return tlsConfig
}

func TestTLSEncoding(t *testing.T) {

	tlsConfig := genTLSConfig(t)

	for _, buffer := range tlsConfig.buffers {
		referenceBytes := make([]byte, buffer.Len())
		copy(referenceBytes, buffer.Bytes())

		if err := buffer.Encode(); err != nil {
			t.Errorf("Failed encoding buffer: %v", err)
			continue
		}

		b64Reader := base64.NewDecoder(base64.StdEncoding, buffer)

		gzipReader, err := gzip.NewReader(b64Reader)
		if err != nil {
			t.Errorf("Failed creating gzip decoder %s: %v", buffer.Name, err)
			continue
		}

		decoded := &bytes.Buffer{}
		if _, err := io.Copy(decoded, gzipReader); err != nil {
			t.Errorf("Failed decoding gzip %s : %v", buffer.Name, err)
		}

		gzipReader.Close()

		if bytes.Compare(decoded.Bytes(), referenceBytes) != 0 {
			t.Logf("decoded:\n%s\n\n", decoded.String())
			t.Logf("reference:\n%s\n\n", referenceBytes)
			t.Errorf("Decoded bytes differ for %s", buffer.Name)
		}
	}

}

func TestTLSGeneration(t *testing.T) {
	tlsConfig := genTLSConfig(t)

	pairs := []*struct {
		KeyBuffer  *blobutil.NamedBuffer
		CertBuffer *blobutil.NamedBuffer
		Key        *rsa.PrivateKey
		Cert       *x509.Certificate
	}{
		//CA MUST come first
		{
			KeyBuffer:  tlsConfig.CAKey,
			CertBuffer: tlsConfig.CACert,
		},
		{
			KeyBuffer:  tlsConfig.APIServerKey,
			CertBuffer: tlsConfig.APIServerCert,
		},
		{
			KeyBuffer:  tlsConfig.AdminKey,
			CertBuffer: tlsConfig.AdminCert,
		},
		{
			KeyBuffer:  tlsConfig.WorkerKey,
			CertBuffer: tlsConfig.WorkerCert,
		},
	}

	var err error
	for _, pair := range pairs {

		if keyBlock, _ := pem.Decode(pair.KeyBuffer.Bytes()); keyBlock == nil {
			t.Errorf("Failed decoding pem block from %s", pair.KeyBuffer.Name)
		} else {
			pair.Key, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
			if err != nil {
				t.Errorf("Failed to parse key %s : %v", pair.KeyBuffer.Name, err)
			}
		}

		if certBlock, _ := pem.Decode(pair.CertBuffer.Bytes()); certBlock == nil {
			t.Errorf("Failed decoding pem block from %s", pair.CertBuffer.Name)
		} else {
			pair.Cert, err = x509.ParseCertificate(certBlock.Bytes)
			if err != nil {
				t.Errorf("Failed to parse cert %s: %v", pair.KeyBuffer.Name, err)
			}
		}
	}

	t.Log("TLS assets parsed successfully")

	if t.Failed() {
		t.Fatalf("TLS key pairs not parsed, cannot verify signatures")
	}

	caCert := pairs[0].Cert
	for _, pair := range pairs[1:] {
		if err := pair.Cert.CheckSignatureFrom(caCert); err != nil {
			t.Errorf(
				"Could not verify ca signature %s : %v",
				pair.CertBuffer.Name,
				err)
		}
	}
}
