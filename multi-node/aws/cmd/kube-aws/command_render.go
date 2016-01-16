package main

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/tlsutil"
	"github.com/spf13/cobra"
)

var (
	cmdRender = &cobra.Command{
		Use:   "render",
		Short: "Render a CloudFormation template",
		Long:  ``,
		Run:   runCmdRender,
	}

	renderOpts struct {
		ConfigPath string
	}
	kubeconfigTemplate *template.Template
)

func init() {
	kubeconfigTemplate = template.Must(template.New("kubeconfig").Parse(kubeconfigTemplateContents))

	cmdRoot.AddCommand(cmdRender)
	cmdRender.Flags().StringVar(&renderOpts.ConfigPath, "config", "./cluster.yaml", "Path to config yaml file")
}

func runCmdRender(cmd *cobra.Command, args []string) {
	cfg := cluster.NewDefaultConfig(VERSION)
	err := cluster.DecodeConfigFromFile(cfg, renderOpts.ConfigPath)
	if err != nil {
		stderr("Unable to load cluster config: %v", err)
		os.Exit(1)
	}

	if rootOpts.AssetDir == "" {
		stderr("--asset-dir option is not specified")
		os.Exit(1)
	}
	if err := initAssetDirectory(cfg); err != nil {
		stderr("Error initializing asset directory: %v", err)
		os.Exit(1)
	}

	tmpl, err := cluster.StackTemplateBody()
	if err != nil {
		stderr("Failed to generate template: %v", err)
		os.Exit(1)
	}

	templatePath := filepath.Join(rootOpts.AssetDir, "template.json")
	if err := ioutil.WriteFile(templatePath, []byte(tmpl), 0600); err != nil {
		stderr("Failed writing output to %s: %v", templatePath, err)
		os.Exit(1)
	}
}

func initAssetDirectory(cfg *cluster.Config) error {
	if _, err := os.Stat(rootOpts.AssetDir); err == nil {
		return fmt.Errorf("Asset directory %s already exists!", rootOpts.AssetDir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("Error stat-ing asset directory path %s: %v", rootOpts.AssetDir, err)
	}

	if err := copyDir(rootOpts.AssetDir, "./artifacts"); err != nil {
		return fmt.Errorf("Error copying artifacts: %v", err)
	}

	inCfg, err := os.Open(renderOpts.ConfigPath)
	if err != nil {
		return err
	}
	defer inCfg.Close()

	outCfgPath := filepath.Join(rootOpts.AssetDir, "cluster.yaml")
	outCfg, err := os.OpenFile(outCfgPath, os.O_WRONLY|os.O_CREATE, 0700)
	if err != nil {
		return err
	}
	defer outCfg.Close()

	if _, err := io.Copy(outCfg, inCfg); err != nil {
		return fmt.Errorf("Error copying config: %v", err)
	}

	credentialsDir := filepath.Join(rootOpts.AssetDir, "credentials")
	if err := os.Mkdir(credentialsDir, 0700); err != nil {
		return fmt.Errorf("Error creating credentials directory %s: %v", credentialsDir, err)
	}

	if err := initTLS(cfg, credentialsDir); err != nil {
		stderr("Failed initializing TLS infrastructure: %v", err)
		os.Exit(1)
	}

	fmt.Println("Initialized TLS infrastructure")

	kubeconfig, err := newKubeconfig(cfg)
	if err != nil {
		stderr("Failed rendering kubeconfig: %v", err)
		os.Exit(1)
	}

	kubeconfigPath := path.Join(credentialsDir, "kubeconfig")
	if err := ioutil.WriteFile(kubeconfigPath, kubeconfig, 0600); err != nil {
		stderr("Failed writing kubeconfig to %s: %v", kubeconfigPath, err)
		os.Exit(1)
	}

	fmt.Printf("Wrote kubeconfig to %s\n", kubeconfigPath)

	return nil
}

func initTLS(cfg *cluster.Config, tlsDir string) error {
	tlsConfig := cluster.NewTLSConfig(tlsDir)

	caConfig := tlsutil.CACertConfig{
		CommonName:   "kube-ca",
		Organization: "kube-aws",
	}
	caKey, caCert, err := initTLSCA(caConfig, tlsConfig.CAKeyFile, tlsConfig.CACertFile)
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
	if err := initTLSServer(apiserverConfig, caCert, caKey, tlsConfig.APIServerKeyFile, tlsConfig.APIServerCertFile); err != nil {
		return err
	}

	workerConfig := tlsutil.ClientCertConfig{
		CommonName: "kube-worker",
		DNSNames: []string{
			"*.*.compute.internal", // *.<region>.compute.internal
			"*.ec2.internal",       // for us-east-1
		},
	}
	if err := initTLSClient(workerConfig, caCert, caKey, tlsConfig.WorkerKeyFile, tlsConfig.WorkerCertFile); err != nil {
		return err
	}

	adminConfig := tlsutil.ClientCertConfig{
		CommonName: "kube-admin",
	}
	if err := initTLSClient(adminConfig, caCert, caKey, tlsConfig.AdminKeyFile, tlsConfig.AdminCertFile); err != nil {
		return err
	}

	return nil
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

func newKubeconfig(cfg *cluster.Config) ([]byte, error) {
	data := struct {
		ClusterName       string
		APIServerEndpoint string
		AdminCertFile     string
		AdminKeyFile      string
		CACertFile        string
	}{
		ClusterName:       cfg.ClusterName,
		APIServerEndpoint: fmt.Sprintf("https://%s", cfg.ExternalDNSName),
		AdminCertFile:     "admin.pem",
		AdminKeyFile:      "admin-key.pem",
		CACertFile:        "ca.pem",
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

func copyDir(target, src string) error {
	walker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(target, relPath)
		if err != nil {
			return err
		}

		if info.IsDir() {
			if err := os.Mkdir(targetPath, 0700); err != nil {
				return err
			}
		} else {
			srcFile, err := os.Open(path)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			targetFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			defer targetFile.Close()

			if _, err := io.Copy(targetFile, srcFile); err != nil {
				return err
			}
		}

		return nil
	}
	return filepath.Walk(src, walker)
}
