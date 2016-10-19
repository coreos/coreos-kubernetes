# Users of kube-aws

## C-SATS

Adam Monsen <adam.monsen@csats.com> @meonkeys

**Machines count:** ~4 nodes, ~100 containers

**Use case:** we originally chose Kubernetes because we needed to manage and be
able to scale out Meteor apps. Galaxy did not yet exist. Kubernetes also
appeared to have more (and more useful) features than Amazon ECS. We also
require HIPAA-compliant storage (e.g. at-rest strong encryption), which Galaxy
also does/did not offer. Today we use Kubernetes for running all our cattle and
pets. We first tried Kubernetes on Ubuntu server EC2 instances, but eventually
found kube-aws way easier and more stable. @iameli set up most of our cluster
infrastructure, and I recall this was prompted in part by meeting with
CoreOS/Tectonic folks at a Kubecon.

## Descomplica

Daniel Martins <daniel.martins@descomplica.com.br>

**Machines count:** ~4 nodes, ~50 containers

**Use Case:** We were using Elastic Beanstalk for all our applications, but it
was very hard to fully utilize the available compute resources since each
Beanstalk instance runs one application only. We considered using ECS as a way
to run many containers per instance, but ECS still isn't available in our main
AWS region (sa-east-1), so it wasn't really an option.

When first testing Kubernetes as a possible solution for that problem, I played
around with kube-up but didn't quite like the way kube-up worked. After looking
around a bit, I found kube-aws and although it missed a few features I wanted
(i.e. cluster level logging), I really dig the fact it uses CloudFormation
under the hoods. It really made pretty easy for me to change the things I
wanted.

So after a few months I managed to migrate all our staging environments to
Kubernetes and last week I moved the first project (a NodeJS front-end
application) to production. 🎉

I also put together a AWS Lambda function (nothing fancy, just a couple hundred
lines of JS code) to automate the deploy of our applications to the Kubernetes
cluster via kubectl based on GitHub and CircleCI notifications.

One particularly nice component of our deployment pipeline is what we call here
development environments; every time someone submits a pull request, this
Lambda function creates a temporary deployment in Kubernetes so that other
developers can see the change "live" and give a more accurate review on the
changes being made. Then, when the pull request is merged/closed, the
deployment is automatically deleted.

## Entelo

Tom Benner @tombenner

**Use case:** Data pipeline scaling system designed for use by engineering
team. Custom built deployment tools are built on top of Kubernetes by the
Entelo team.

## Stream Kitchen

Eli Mallon <eli@stream.kitchen> @iameli

**Machines count:** Varies wildly depending on what's being processed that day

**Use case:** kube-aws provides a robust, scalable foundation for compositing
live video streams in the cloud utilizes spot instances to dynamically shift
processing onto the cheapest hardware currently available easy CloudFormation
bootstrapping allows us to come up in new regions without fuss.
