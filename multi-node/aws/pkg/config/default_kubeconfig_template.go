package config

var defaultKubeConfigTemplate = `apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: {{ .TLSConfig.CACert.Name }}
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
    client-certificate: {{ .TLSConfig.AdminCert.Name }}
    client-key: {{ .TLSConfig.AdminKey.Name }}
current-context: kube-aws-{{ .ClusterName }}-context
`
