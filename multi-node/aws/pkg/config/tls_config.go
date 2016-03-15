package config

import (
	"crypto/rsa"
	"crypto/x509"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/blobutil"
	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/tlsutil"
)

type TLSConfig struct {
	CACert *blobutil.NamedBuffer
	CAKey  *blobutil.NamedBuffer

	APIServerCert *blobutil.NamedBuffer
	APIServerKey  *blobutil.NamedBuffer

	WorkerCert *blobutil.NamedBuffer
	WorkerKey  *blobutil.NamedBuffer

	AdminCert *blobutil.NamedBuffer
	AdminKey  *blobutil.NamedBuffer

	buffers blobutil.NamedBufferList
}

func newTLSConfig() *TLSConfig {
	tlsConfig := &TLSConfig{
		CACert: &blobutil.NamedBuffer{Name: "ca.pem"},
		CAKey:  &blobutil.NamedBuffer{Name: "ca-key.pem"},

		APIServerCert: &blobutil.NamedBuffer{Name: "apiserver.pem"},
		APIServerKey:  &blobutil.NamedBuffer{Name: "apiserver-key.pem"},

		WorkerCert: &blobutil.NamedBuffer{Name: "worker.pem"},
		WorkerKey:  &blobutil.NamedBuffer{Name: "worker-key.pem"},

		AdminCert: &blobutil.NamedBuffer{Name: "admin.pem"},
		AdminKey:  &blobutil.NamedBuffer{Name: "admin-key.pem"},
	}

	tlsConfig.buffers = blobutil.NamedBufferList{
		tlsConfig.CACert,
		tlsConfig.CAKey,

		tlsConfig.APIServerCert,
		tlsConfig.APIServerKey,

		tlsConfig.WorkerCert,
		tlsConfig.WorkerKey,

		tlsConfig.AdminCert,
		tlsConfig.AdminKey,
	}

	return tlsConfig
}

func (tc *TLSConfig) generateAllTLS(cfg *Config) error {

	caConfig := tlsutil.CACertConfig{
		CommonName:   "kube-ca",
		Organization: "kube-aws",
	}

	caKey, caCert, err := tc.generateTLSCA(caConfig)
	if err != nil {
		return err
	}

	apiserverConfig := tlsutil.ServerCertConfig{
		CommonName: "kube-apiserver",
		DNSNames: []string{
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			"kubernetes.default.svc.cluster.local",
			cfg.ExternalDNSName,
		},
		IPAddresses: []string{
			cfg.ControllerIP,
			cfg.KubernetesServiceIP,
		},
	}
	if err := tc.generateTLSServer(apiserverConfig, caKey, caCert); err != nil {
		return err
	}

	workerConfig := tlsutil.ClientCertConfig{
		CommonName: "kube-worker",
		DNSNames: []string{
			"*.*.compute.internal",
			"*.ec2.internal",
		},
	}
	if err := tc.generateTLSClientWorker(workerConfig, caKey, caCert); err != nil {
		return err
	}

	adminConfig := tlsutil.ClientCertConfig{
		CommonName: "kube-admin",
	}
	return tc.generateTLSClientAdmin(adminConfig, caKey, caCert)
}

func (tc *TLSConfig) generateTLSCA(cfg tlsutil.CACertConfig) (*x509.Certificate, *rsa.PrivateKey, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	cert, err := tlsutil.NewSelfSignedCACertificate(cfg, key)
	if err != nil {
		return nil, nil, err
	}

	if err := tlsutil.WritePrivateKeyPEMBlock(tc.CAKey, key); err != nil {
		return nil, nil, err
	}
	if err := tlsutil.WriteCertificatePEMBlock(tc.CACert, cert); err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

func (tc *TLSConfig) generateTLSServer(cfg tlsutil.ServerCertConfig, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return err
	}

	cert, err := tlsutil.NewSignedServerCertificate(cfg, key, caCert, caKey)
	if err != nil {
		return err
	}

	if err := tlsutil.WritePrivateKeyPEMBlock(tc.APIServerKey, key); err != nil {
		return err
	}
	if err := tlsutil.WriteCertificatePEMBlock(tc.APIServerCert, cert); err != nil {
		return err
	}

	return nil
}

func (tc *TLSConfig) generateTLSClientAdmin(cfg tlsutil.ClientCertConfig, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return err
	}

	cert, err := tlsutil.NewSignedClientCertificate(cfg, key, caCert, caKey)
	if err != nil {
		return err
	}

	if err := tlsutil.WritePrivateKeyPEMBlock(tc.AdminKey, key); err != nil {
		return err
	}
	if err := tlsutil.WriteCertificatePEMBlock(tc.AdminCert, cert); err != nil {
		return err
	}

	return nil
}

func (tc *TLSConfig) generateTLSClientWorker(cfg tlsutil.ClientCertConfig, caCert *x509.Certificate, caKey *rsa.PrivateKey) error {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return err
	}

	cert, err := tlsutil.NewSignedClientCertificate(cfg, key, caCert, caKey)
	if err != nil {
		return err
	}

	if err := tlsutil.WritePrivateKeyPEMBlock(tc.WorkerKey, key); err != nil {
		return err
	}
	if err := tlsutil.WriteCertificatePEMBlock(tc.WorkerCert, cert); err != nil {
		return err
	}

	return nil
}
