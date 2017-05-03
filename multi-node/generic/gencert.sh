#!/usr/bin/env bash
#
# Generates certs using 'cfssl'
# Instructions taken from: https://coreos.com/os/docs/latest/generate-self-signed-certificates.html
# Print cert (openssl): openssl x509 -in ca.pem -text -noout
# Print cert (cfssl): cfssl certinfo -cert ca.pem

declare -A humans

#### Config
apiserver_fqdn=con.kube  #the DNS "A" record that round-robins all master nodes
api_service_IP=10.3.0.1

controllers=(con{0..2}.kube)
workers=(work{0..2}.kube)
humans=(["admin"]="system:masters" ["dude"]="group1,group2,group3")

#### Helpers
error() {
  echo "$1 Exiting."
  exit 1
}

toolcheck() {
  which dig &> /dev/null || error "'dig' not found."
  which cfssl &> /dev/null || error "'cfssl' not found."
  which cfssljson &> /dev/null || error "'cfssljson' not found."
}

# homebrew's default cfssl binary doesn't support multiple Organizations
# in the cert, although it does for the CSR. This is because it was compiled
# against Go 1.7. See https://github.com/cloudflare/cfssl/issues/599
#
# Run these commands to compile 'cfssl' against Go 1.8
#   brew remove cfssl
#   brew upgrade go
#   brew install --build-from-source cfssl
gen_certs() {
  local element="$1"
  local name="$2"
  local groups="$3"

  # Set profile and other variables based on element type
  local app_cert_dir="kubernetes"  #default app subfolder for certs is "kubernetes"
  local cn="$name"  #CN is the hostname/username by default
  local profile=""
  local alt_names=""
  case $element in
    etcd )
      profile=peer
      app_cert_dir="etcd"
      alt_names="$name,$(dig +short $name),$apiserver_fqdn"
      ;;
    flannel )
      profile=client
      app_cert_dir="flannel"
      ;;
    calico )
      profile=client
      app_cert_dir="calico"
      ;;
    kube-apiserver_server )
      profile=server
      alt_names="$name,$(dig +short $name),$apiserver_fqdn,$api_service_IP,localhost,127.0.0.1,kubernetes,kubernetes.default,kubernetes.default.svc,kubernetes.default.svc.cluster.local"
      ;;
    kube-apiserver_client )
      profile=client
      ;;
    kubelet_server )
      profile=server
      alt_names="$name,$(dig +short $name)"
      ;;
    kubelet_client )
      profile=client
      cn="system:node:$controller"
      groups="system:nodes" #overwrite the original groups
      ;;
    kube-controller-manager )
      profile=client
      cn="system:kube-controller-manager"
      ;;
    kube-scheduler )
      profile=client
      cn="system:kube-scheduler"
      ;;
    kube-proxy )
      profile=client
      cn="system:kube-proxy"
      ;;
    person )
      profile=client
      ;;
    * )
      echo "'$element' doesn't match: etcd, kubelet_server, kubelet_client, kube-apiserver_server, kube-apiserver_client, kube-controller-manager, kube-scheduler, kube-proxy, person. Skipping."
      return
      ;;
  esac

  # Configure cert parameters
  element_dir=$(cut -d_ -f1 <<< "$element")
  local dst_dir="$name/$app_cert_dir/$element_dir"
  if [ "$element" == "person" ]; then #no subfolders for client certs for a "person"
    dst_dir="$name"
  elif [ "$element" == "etcd" ] || [ "$element" == "flannel" ] || [ "$element" == "calico" ]; then  #etcd/calico/flannel doesn't need yet another subfolder
      dst_dir="$name/$element_dir"
  fi
  local certname="$profile.pem"
  local keyname="$profile-key.pem"

  # Check if certs have been generated
  if [ -e "$dst_dir/$keyname" ] || [ -e "$dst_dir/$certname" ]; then
    printf "Exists"
    return
  fi

  # Add groups only if it's a client cert
  local groups_json=""
  if [ "$profile" == "client" ] && [ -n "$groups" ]; then
    groups_json='"names":['
    IFS=,; for group in $groups; do
      groups_json="$groups_json"'{"O":"'$group'"},'
    done
    groups_json=",${groups_json:0:${#groups_json}-1}"']'
  fi

  echo '{"CN":"'$cn'"'"$groups_json"',"hosts":[""],"key":{"algo":"rsa","size":2048}}' |\
    cfssl -loglevel 5 gencert \
    -ca=ca.pem \
    -ca-key=ca-key.pem \
    -config=ca-config.json \
    -profile=$profile \
    -hostname="$alt_names" - |\
    cfssljson -bare $profile

  # Rename files to the component's name
  mkdir -p $dst_dir
  cp -n ca.pem $name/$app_cert_dir
  cp -n ca.pem $dst_dir
  rm $profile.csr
  mv $profile.pem $dst_dir/$certname
  chmod 400 $profile-key.pem
  mv $profile-key.pem $dst_dir/$keyname

  printf "Done"
}


#### Main
## Check that the tools are present
toolcheck

## Generate CA cert if ca.pem or ca-key.pem are absent
printf "+ Generate CA cert: "
if !( [ -e ca-key.pem ] && [ -e ca.pem ] ); then
  cfssl -loglevel 5 gencert -initca ca-csr.json | cfssljson -bare ca -
  printf "Done\n"
else
  printf "Exists\n"
fi

printf "\n+ Generate ServiceAccount cert: "
if !( [ -e serviceaccount-key.pem ] && [ -e serviceaccount.pem ] ); then
  echo '{"CN":"serviceaccount","hosts":[""],"key":{"algo":"rsa","size":2048}}' |\
    cfssl -loglevel 5 gencert \
    -ca=ca.pem \
    -ca-key=ca-key.pem \
    -config=ca-config.json \
    -profile=server - |\
    cfssljson -bare server

  # Rename files according to its usage
  rm server.csr
  mv server.pem serviceaccount.pem
  chmod 400 server-key.pem
  mv server-key.pem serviceaccount-key.pem
  printf "Done\n"
else
  printf "Exists\n"
fi

printf "\n+ Generate controller node certs\n"
for controller in ${controllers[@]}; do
  elements=("etcd" "flannel" "calico" "kube-apiserver_server" "kube-apiserver_client" "kubelet_server" "kubelet_client" "kube-controller-manager" "kube-scheduler" "kube-proxy")
  printf "[$controller]"

  for element in ${elements[@]}; do
    printf " $element:"
    gen_certs $element $controller
  done
  printf "\n"

  # Copy ServiceAccount certs to the controllers
  apiserver_dir="$controller/kubernetes/kube-apiserver"
  if [ -d $apiserver_dir ]; then
    rm -f $apiserver_dir/serviceaccount*.pem #cannot overwrite serviceaccount-key.pem due to permissions
    cp serviceaccount*.pem $apiserver_dir
  fi
done

printf "Done\n"

printf "\n+ Generate worker node certs\n"
for worker in ${workers[@]}; do
  elements=("flannel" "calico" "kubelet_server" "kubelet_client" "kube-proxy")
  printf "[$worker]"

  for element in ${elements[@]}; do
    printf " $element:"
    gen_certs $element $worker
  done
  printf "\n"
done

printf "\n+ Generate human certs\n"
for person in ${!humans[@]}; do
  printf "$person:"
  gen_certs person $person ${humans[$person]} 
  printf "\n"
done
