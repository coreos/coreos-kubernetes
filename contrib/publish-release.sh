#!/bin/bash -e
#
# Example usage:
#
# $ export S3_BUCKET=kube-aws-v0.2.0-patch
# $ aws s3 mb $S3_BUCKET
# make_bucket: s3://kube-aws-v0.2.0-patch/
# $ contrib/publish-release.sh v0.2.0-patch
# Building kube-aws a3d5cdb560cc94cb4b1c65bbc5db24a2077f1fd1...
# ...
# Finished building release artifacts for v0.2.0-patch
# $ ls release/
# kube-aws-darwin-amd64.tar.gz kube-aws-linux-amd64.tar.gz

function usage {
   echo "$0 <version>"
}

if [ $# -ne 1 ]; then
	usage
	exit 2
fi

VERSION=$1
S3_BUCKET=${S3_BUCKET:-coreos-kubernetes}

echo "Preparing release artifacts for $VERSION"

pushd multi-node/aws

./build
echo "Built kube-aws binary for local system"

./bin/kube-aws render --output=artifacts/template.json
echo "Generated CloudFormation template"

aws s3 cp --recursive --acl=public-read artifacts/ s3://${S3_BUCKET}/${VERSION}
aws s3 cp --recursive --acl=public-read artifacts/ s3://${S3_BUCKET}/latest
echo "Copied artifacts to S3 bucket"

popd

rm -fr release/

oss=( "linux" "darwin" )
for os in ${oss[@]}; do
	pushd multi-node/aws
	GOOS=$os ./build
	popd

	mkdir -p release/kube-aws-$os-amd64
	mv multi-node/aws/bin/kube-aws release/kube-aws-$os-amd64/
	tar -C release/kube-aws-$os-amd64/ -czf release/kube-aws-$os-amd64.tar.gz kube-aws
	rm -r release/kube-aws-$os-amd64

	echo "Built release artifact for $os"
done

echo "Finished building release artifacts for $VERSION"
