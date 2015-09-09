# Basic overview

This sets up a basic master, with up to X worker nodes.

## Prepare required keys material and config files

## Configure and install systemd units

## Setup Flannel

## Setup Docker

## Setup pods for master using kubelet

This is for a single master. Single master is OK because if the master goes away, nothing stops in the cluster-- except new scheduling. However, we have also written a [guide for creating a highly available master].

## Setup pods for worker nodes kubelet

## Start kubelet

## Start addons once API is up

## Configure kubectl locally

