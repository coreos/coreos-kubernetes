package main

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"

	"archive/tar"
	"compress/gzip"
	"encoding/base64"
	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/tlsutil"
	"io"
)

var (
	cmdUp = &cobra.Command{
		Use:   "up",
		Short: "Create a new Kubernetes cluster",
		Long:  ``,
		Run:   runCmdUp,
	}

	kubeconfigTemplate *template.Template
)

func init() {
	kubeconfigTemplate = template.Must(template.New("kubeconfig").Parse(kubeconfigTemplateContents))

	cmdRoot.AddCommand(cmdUp)
}

func runCmdUp(cmd *cobra.Command, args []string) {
	cfg := cluster.NewDefaultConfig(VERSION)
	err := cluster.DecodeConfigFromFile(cfg, rootOpts.ConfigPath)
	if err != nil {
		stderr("Unable to load cluster config: %v", err)
		os.Exit(1)
	}

	c := cluster.New(cfg, newAWSConfig(cfg))

	clusterDir, err := filepath.Abs(path.Join("clusters", cfg.ClusterName))
	if err != nil {
		stderr("Unable to expand cluster directory to absolute path: %v", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(clusterDir, 0700); err != nil {
		stderr("Failed creating cluster workspace %s: %v", clusterDir, err)
		os.Exit(1)
	}

	artifactsDir, err := filepath.Abs(cfg.ArtifactPath)
	if err != nil {
		stderr("Unable to expand artifacts directory to absolute path: %v", err)
		os.Exit(1)
	}
	if err := initArtifacts(cfg, artifactsDir); err != nil {
		stderr("Failed initializing artifacts from %s: %v", artifactsDir, err)
		os.Exit(1)
	}

	tlsConfig, err := initTLS(cfg, clusterDir)
	if err != nil {
		stderr("Failed initializing TLS infrastructure: %v", err)
		os.Exit(1)
	}

	fmt.Println("Initialized TLS infrastructure")

	kubeconfig, err := newKubeconfig(cfg, tlsConfig)
	if err != nil {
		stderr("Failed rendering kubeconfig: %v", err)
		os.Exit(1)
	}

	kubeconfigPath := path.Join(clusterDir, "kubeconfig")
	if err := ioutil.WriteFile(kubeconfigPath, kubeconfig, 0600); err != nil {
		stderr("Failed writing kubeconfig to %s: %v", kubeconfigPath, err)
		os.Exit(1)
	}

	fmt.Printf("Wrote kubeconfig to %s\n", kubeconfigPath)

	fmt.Println("Waiting for cluster creation...")

	if err := c.Create(tlsConfig); err != nil {
		stderr("Failed creating cluster: %v", err)
		os.Exit(1)
	}

	fmt.Println("Successfully created cluster")
	fmt.Println("")

	info, err := c.Info()
	if err != nil {
		stderr("Failed fetching cluster info: %v", err)
		os.Exit(1)
	}

	fmt.Printf(info.String())
}

func getCloudFormation(url string) (string, error) {
	r, err := http.Get(url)

	if err != nil {
		return "", fmt.Errorf("Failed to get template: %v", err)
	}

	if r.StatusCode != 200 {
		return "", fmt.Errorf("Failed to get template: invalid status code: %d", r.StatusCode)
	}

	tmpl, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to get template: %v", err)
	}
	r.Body.Close()

	return string(tmpl), nil
}
func mustTarDirectory(basepath, path string) *bytes.Buffer {
	buf := &bytes.Buffer{}

	b64Writer := base64.NewEncoder(base64.StdEncoding, buf)
	defer b64Writer.Close()

	gzWriter, err := gzip.NewWriterLevel(b64Writer, gzip.BestCompression)
	if err != nil {
		stderr("Error creating gzip writer: %s", err)
		os.Exit(1)
	}
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	tarHandler := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			//Terminate on error
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(basepath, path)
		if err != nil {
			return err
		}

		hdr.Name = relPath

		if err = tarWriter.WriteHeader(hdr); err != nil {
			return err
		}

		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tarWriter, f); err != nil {
				return err
			}
		}

		return nil
	}

	if err := filepath.Walk(path, tarHandler); err != nil {
		stderr("Error tar-ing directory %s: %s", path, err)
		os.Exit(1)
	}
	return buf
}

func mustReadFile(loc string) *bytes.Buffer {
	f, err := os.Open(loc)
	if err != nil {
		stderr("Failed opening file %s: %s", loc, err)
		os.Exit(1)
	}
	defer f.Close()

	buf := &bytes.Buffer{}

	b64Writer := base64.NewEncoder(base64.StdEncoding, buf)
	defer b64Writer.Close()

	gzWriter, err := gzip.NewWriterLevel(b64Writer, gzip.BestCompression)
	if err != nil {
		stderr("Failed creating gzip context: %s", err)
		os.Exit(1)
	}
	defer gzWriter.Close()

	if _, err := io.Copy(gzWriter, f); err != nil {
		stderr("Failed reading file %s: %s", loc, err)
		os.Exit(1)
	}

	return buf
}

func newKubeconfig(cfg *cluster.Config, tlsConfig *cluster.TLSConfig) ([]byte, error) {
	data := struct {
		ClusterName       string
		APIServerEndpoint string
		AdminCertFile     string
		AdminKeyFile      string
		CACertFile        string
	}{
		ClusterName:       cfg.ClusterName,
		APIServerEndpoint: fmt.Sprintf("https://%s", cfg.ExternalDNSName),
		AdminCertFile:     tlsConfig.AdminCertFile,
		AdminKeyFile:      tlsConfig.AdminKeyFile,
		CACertFile:        tlsConfig.CACertFile,
	}

	var rendered bytes.Buffer
	if err := kubeconfigTemplate.Execute(&rendered, data); err != nil {
		return nil, err
	}

	return rendered.Bytes(), nil
}

var kubeconfigTemplateContents = `apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: {{ .CACertFile }}
    server: {{ .APIServerEndpoint }}
  name: kube-aws-{{ .ClusterName }}-cluster
contexts:
- context:
    cluster: kube-aws-{{ .ClusterName }}-cluster
    namespace: default
    user: kube-aws-{{ .ClusterName }}-admin
  name: kube-aws-{{ .ClusterName }}-context
users:
- name: kube-aws-{{ .ClusterName }}-admin
  user:
    client-certificate: {{ .AdminCertFile }}
    client-key: {{ .AdminKeyFile }}
current-context: kube-aws-{{ .ClusterName }}-context
`

func initArtifacts(cfg *cluster.Config, artifactPath string) error {

	cfg.InstallWorkerScript = mustReadFile(filepath.Join(artifactPath, "scripts", "install-worker.sh")).Bytes()

	cfg.InstallControllerScript = mustReadFile(filepath.Join(artifactPath, "scripts", "install-controller.sh")).Bytes()

	manifestPath := filepath.Join(artifactPath, "manifests")

	cfg.ClusterManifestsTar = mustTarDirectory(artifactPath, filepath.Join(manifestPath, "cluster")).Bytes()
	cfg.ControllerManifestsTar = mustTarDirectory(artifactPath, filepath.Join(manifestPath, "controller")).Bytes()
	cfg.WorkerManifestsTar = mustTarDirectory(artifactPath, filepath.Join(manifestPath, "worker")).Bytes()

	return nil
}
func initTLS(cfg *cluster.Config, dir string) (*cluster.TLSConfig, error) {
	caCertPath := path.Join(dir, "ca.pem")
	caKeyPath := path.Join(dir, "ca-key.pem")
	caConfig := tlsutil.CACertConfig{
		CommonName:   "kube-ca",
		Organization: "kube-aws",
	}
	caKey, caCert, err := initTLSCA(caConfig, caKeyPath, caCertPath)
	if err != nil {
		return nil, err
	}

	apiserverCertPath := path.Join(dir, "apiserver.pem")
	apiserverKeyPath := path.Join(dir, "apiserver-key.pem")
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
	if err := initTLSServer(apiserverConfig, caCert, caKey, apiserverKeyPath, apiserverCertPath); err != nil {
		return nil, err
	}

	workerCertPath := path.Join(dir, "worker.pem")
	workerKeyPath := path.Join(dir, "worker-key.pem")
	workerConfig := tlsutil.ClientCertConfig{
		CommonName: "kube-worker",
		DNSNames: []string{
			"*.*.compute.internal", // *.<region>.compute.internal
			"*.ec2.internal",       // for us-east-1
		},
	}
	if err := initTLSClient(workerConfig, caCert, caKey, workerKeyPath, workerCertPath); err != nil {
		return nil, err
	}

	adminCertPath := path.Join(dir, "admin.pem")
	adminKeyPath := path.Join(dir, "admin-key.pem")
	adminConfig := tlsutil.ClientCertConfig{
		CommonName: "kube-admin",
	}
	if err := initTLSClient(adminConfig, caCert, caKey, adminKeyPath, adminCertPath); err != nil {
		return nil, err
	}

	tlsConfig := cluster.TLSConfig{
		CACertFile:        caCertPath,
		CACert:            mustReadFile(caCertPath).Bytes(),
		APIServerCertFile: apiserverCertPath,
		APIServerCert:     mustReadFile(apiserverCertPath).Bytes(),
		APIServerKeyFile:  apiserverKeyPath,
		APIServerKey:      mustReadFile(apiserverKeyPath).Bytes(),
		WorkerCertFile:    workerCertPath,
		WorkerCert:        mustReadFile(workerCertPath).Bytes(),
		WorkerKeyFile:     workerKeyPath,
		WorkerKey:         mustReadFile(workerKeyPath).Bytes(),
		AdminCertFile:     adminCertPath,
		AdminCert:         mustReadFile(adminCertPath).Bytes(),
		AdminKeyFile:      adminKeyPath,
		AdminKey:          mustReadFile(adminKeyPath).Bytes(),
	}

	return &tlsConfig, nil
}

func initTLSCA(cfg tlsutil.CACertConfig, keyPath, certPath string) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	cert, err := tlsutil.NewSelfSignedCACertificate(cfg, key)
	if err != nil {
		return nil, nil, err
	}

	if err := writeKey(keyPath, key); err != nil {
		return nil, nil, err
	}
	if err := writeCert(certPath, cert); err != nil {
		return nil, nil, err
	}

	return key, cert, nil
}

func initTLSServer(cfg tlsutil.ServerCertConfig, caCert *x509.Certificate, caKey *rsa.PrivateKey, keyPath, certPath string) error {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return err
	}

	cert, err := tlsutil.NewSignedServerCertificate(cfg, key, caCert, caKey)
	if err != nil {
		return err
	}

	if err := writeKey(keyPath, key); err != nil {
		return err
	}
	if err := writeCert(certPath, cert); err != nil {
		return err
	}

	return nil
}

func initTLSClient(cfg tlsutil.ClientCertConfig, caCert *x509.Certificate, caKey *rsa.PrivateKey, keyPath, certPath string) error {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return err
	}

	cert, err := tlsutil.NewSignedClientCertificate(cfg, key, caCert, caKey)
	if err != nil {
		return err
	}

	if err := writeKey(keyPath, key); err != nil {
		return err
	}
	if err := writeCert(certPath, cert); err != nil {
		return err
	}

	return nil
}

func writeCert(certPath string, cert *x509.Certificate) error {
	f, err := os.OpenFile(certPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return tlsutil.WriteCertificatePEMBlock(f, cert)
}

func writeKey(keyPath string, key *rsa.PrivateKey) error {
	f, err := os.OpenFile(keyPath, os.O_CREATE|os.O_WRONLY, 0400)
	if err != nil {
		return err
	}
	defer f.Close()

	return tlsutil.WritePrivateKeyPEMBlock(f, key)
}
