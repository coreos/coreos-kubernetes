#!/bin/bash
#
# Sends cert to the hosts

#### Config
# cert_dir=/etc/kubernetes/ssl  #cert folder on the remote host

#### Helpers
toolcheck() {
  which dig &> /dev/null || error "'dig' not found."
  which cfssl &> /dev/null || error "'cfssl' not found."
}

#### Main
## Check that the tools are present
toolcheck

# Send certs to remote host
for dir in $(find . -type d -maxdepth 1); do
  fqdn=${dir#*/}

  # Skip the "." path that "find" returns
  [ "$fqdn" != "." ] || continue

  # Check if host is valid and is online
  ping -c1 -W1 $fqdn &> /dev/null
  error=$?
  if [ "$error" -eq 2 ]; then
    echo "+ '$fqdn' is offline. Skipping."
    continue
  elif [ "$error" -eq 68 ]; then
    echo "+ '$fqdn' is a person or host doesn't exist. Skipping."
    continue
  fi

  # SCP the certs to the remote host
  echo "+ Sending certs to $fqdn"
  pushd $dir > /dev/null
  for folder in *; do
    cert_dir="/etc/$folder/ssl"
    scp -r $folder core@$fqdn:~/
    ssh -t core@$fqdn "sudo mkdir -p $cert_dir; sudo rsync -av ~/$folder/ $cert_dir/; sudo chown -R root:root $cert_dir; rm -rf ~/$folder"
    
    # etcd-wrapper runs as "etcd" user so the certs need to be owned by the user
    if [ $folder == "etcd" ]; then
      ssh -t core@$fqdn "sudo chown -R etcd:etcd $cert_dir"
    fi
  done
  popd > /dev/null
done
