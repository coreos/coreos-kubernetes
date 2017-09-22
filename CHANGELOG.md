<div class="k8s-on-tectonic">
<p class="k8s-on-tectonic-description">This repo is not in alignment with current versions of Kubernetes, and will not be active in the future. The CoreOS Kubernetes documentation has been moved to the [tectonic-docs repo](https://github.com/coreos/tectonic-docs/tree/master/Documentation), where it will be published and updated.</p>

<p class="k8s-on-tectonic-description">For tested, maintained, and production-ready Kubernetes instructions, see our [Tectonic Installer documentation](https://coreos.com/tectonic/docs/latest/install/aws/index.html). The Tectonic Installer provides a Terraform-based Kubernetes installation. It is open source, uses upstream Kubernetes and can be easily customized.</p>
</div>

## v0.2.0

- Bump Kubernetes version to v1.1.1
- Use iptables proxy mode in kube-proxy

## v0.1.1

- Use flannel vxlan backend (#113)
- Configure worker certificates with IP SANs (#121)
- Add bare metal documentation (#107)

## v0.1.0

- Multi-Node Installers: Vagrant & AWS
- Single-Node Installer: Vagrant
- Generic Multi-Node Install Guide
