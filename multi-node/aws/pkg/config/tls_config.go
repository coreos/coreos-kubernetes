package config

import (
	"bytes"
	"compress/gzip"
	"crypto/rsa"
	"encoding/base64"
	"io/ioutil"
	"path/filepath"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/tlsutil"
)

// PEM encoded TLS assets.
type RawTLSAssets struct {
	CACert        []byte
	CAKey         []byte
	APIServerCert []byte
	APIServerKey  []byte
	WorkerCert    []byte
	WorkerKey     []byte
	AdminCert     []byte
	AdminKey      []byte
}

// PEM -> gzip -> base64 encoded TLS assets.
type CompactTLSAssets struct {
	CACert        string
	CAKey         string
	APIServerCert string
	APIServerKey  string
	WorkerCert    string
	WorkerKey     string
	AdminCert     string
	AdminKey      string
}

func (c *Cluster) NewTLSAssets() (*RawTLSAssets, error) {
	// Generate keys for the various components.
	keys := make([]*rsa.PrivateKey, 4)
	var err error
	for i := range keys {
		if keys[i], err = tlsutil.NewPrivateKey(); err != nil {
			return nil, err
		}
	}
	caKey, apiServerKey, workerKey, adminKey := keys[0], keys[1], keys[2], keys[3]

	caConfig := tlsutil.CACertConfig{
		CommonName:   "kube-ca",
		Organization: "kube-aws",
	}
	caCert, err := tlsutil.NewSelfSignedCACertificate(caConfig, caKey)
	if err != nil {
		return nil, err
	}

	apiServerConfig := tlsutil.ServerCertConfig{
		CommonName: "kube-apiserver",
		DNSNames: []string{
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			"kubernetes.default.svc.cluster.local",
			c.ExternalDNSName,
		},
		IPAddresses: []string{
			c.ControllerIP,
			c.KubernetesServiceIP,
		},
	}
	apiServerCert, err := tlsutil.NewSignedServerCertificate(apiServerConfig, apiServerKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	workerConfig := tlsutil.ClientCertConfig{
		CommonName: "kube-worker",
		DNSNames: []string{
			"*.*.compute.internal",
			"*.ec2.internal",
		},
	}
	workerCert, err := tlsutil.NewSignedClientCertificate(workerConfig, workerKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	adminConfig := tlsutil.ClientCertConfig{
		CommonName: "kube-admin",
	}
	adminCert, err := tlsutil.NewSignedClientCertificate(adminConfig, adminKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	return &RawTLSAssets{
		CACert:        tlsutil.EncodeCertificatePEM(caCert),
		APIServerCert: tlsutil.EncodeCertificatePEM(apiServerCert),
		WorkerCert:    tlsutil.EncodeCertificatePEM(workerCert),
		AdminCert:     tlsutil.EncodeCertificatePEM(adminCert),
		CAKey:         tlsutil.EncodePrivateKeyPEM(caKey),
		APIServerKey:  tlsutil.EncodePrivateKeyPEM(apiServerKey),
		WorkerKey:     tlsutil.EncodePrivateKeyPEM(workerKey),
		AdminKey:      tlsutil.EncodePrivateKeyPEM(adminKey),
	}, nil
}

func ReadTLSAssets(dirname string) (*RawTLSAssets, error) {
	r := new(RawTLSAssets)
	files := []struct {
		name      string
		cert, key *[]byte
	}{
		{"ca", &r.CACert, &r.CAKey},
		{"apiserver", &r.APIServerCert, &r.APIServerKey},
		{"worker", &r.WorkerCert, &r.WorkerKey},
		{"admin", &r.AdminCert, &r.AdminKey},
	}
	for _, file := range files {
		certPath := filepath.Join(dirname, file.name+".pem")
		keyPath := filepath.Join(dirname, file.name+"-key.pem")

		certData, err := ioutil.ReadFile(certPath)
		if err != nil {
			return nil, err
		}
		*file.cert = certData
		keyData, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return nil, err
		}
		*file.key = keyData
	}
	return r, nil
}

func (r *RawTLSAssets) WriteToDir(dirname string) error {
	assets := []struct {
		name      string
		cert, key []byte
	}{
		{"ca", r.CACert, r.CAKey},
		{"apiserver", r.APIServerCert, r.APIServerKey},
		{"worker", r.WorkerCert, r.WorkerKey},
		{"admin", r.AdminCert, r.AdminKey},
	}
	for _, asset := range assets {
		certPath := filepath.Join(dirname, asset.name+".pem")
		keyPath := filepath.Join(dirname, asset.name+"-key.pem")
		if err := ioutil.WriteFile(certPath, asset.cert, 0600); err != nil {
			return err
		}
		if err := ioutil.WriteFile(keyPath, asset.key, 0600); err != nil {
			return err
		}
	}
	return nil
}

func compressData(d []byte) (string, error) {
	var buff bytes.Buffer
	gzw := gzip.NewWriter(&buff)
	if _, err := gzw.Write(d); err != nil {
		return "", err
	}
	if err := gzw.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buff.Bytes()), nil
}

func (r *RawTLSAssets) Compact() (*CompactTLSAssets, error) {
	var err error
	compact := func(data []byte) string {
		if err != nil {
			return ""
		}
		var out string
		out, err = compressData(data)
		if err != nil {
			return ""
		}
		return out
	}
	compactAssets := CompactTLSAssets{
		CACert:        compact(r.CACert),
		CAKey:         compact(r.CAKey),
		APIServerCert: compact(r.APIServerCert),
		APIServerKey:  compact(r.APIServerKey),
		WorkerCert:    compact(r.WorkerCert),
		WorkerKey:     compact(r.WorkerKey),
		AdminCert:     compact(r.AdminCert),
		AdminKey:      compact(r.AdminKey),
	}
	if err != nil {
		return nil, err
	}
	return &compactAssets, nil
}
