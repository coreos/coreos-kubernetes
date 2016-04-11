# define location of openssl binary manually since running this
# script under Vagrant fails on some systems without it
$OPENSSL="openssl"

function usage {
	$script=split-path $MyInvocation.PSCommandPath -Leaf
    write-host "USAGE: $script <output-dir>"
    write-host "  example: $script .\ssl\ca.pem"
}

if($args[0] -eq $null){
	usage
	return
}

$OUTDIR=$args[0]

if(! (test-path $OUTDIR)){
	write-error "ERROR: output directory does not exist:  $OUTDIR"
	return
}

$OUTFILE="$OUTDIR\ca.pem"

if(test-path $OUTFILE){
	return
}

#Needed to avoid "unable to write 'random state'" error
set-item -force -path env:RANDFILE -value ".rnd"

# establish cluster CA and self-sign a cert
. $OPENSSL genrsa -out "$OUTDIR\ca-key.pem" 2048
. $OPENSSL req -x509 -new -nodes -key "$OUTDIR\ca-key.pem" -days 10000 -out "$OUTFILE" -subj "/CN=kube-ca"