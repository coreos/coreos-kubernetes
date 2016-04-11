# define location of openssl binary manually since running this
# script under Vagrant fails on some systems without it
$OPENSSL="openssl"

function usage {
	$script=split-path $MyInvocation.PSCommandPath -Leaf
    write-host "USAGE: $script <output-dir> <cert-base-name> <CN> [SAN,SAN,SAN]"
    write-host "  example: $script .\ssl\ worker kube-worker IP.1=127.0.0.1,IP.2=10.0.0.1"
}

if($args[0] -eq $null -or $args[1] -eq $null -or $args[2] -eq $null){
	usage
	return
}

$OUTDIR=$args[0]
$CERTBASE=$args[1]
$CN=$args[2]
$SANS=$args[3]

if(!(Get-Command "7z" -errorAction SilentlyContinue)){
	if(test-path "C:\Program Files\7-Zip\7z.exe"){
		$ZIP="C:\Program Files\7-Zip\7z.exe"
	}else{
		if(test-path "C:\Program Files (x86)\7-Zip\7z.exe"){
			$ZIP="C:\Program Files\7-Zip\7z.exe"
		}else{
			write-error "7zip is required to proceed."
			return
		}
	}
}else{ $ZIP="7z" }

if(! (test-path $OUTDIR)){
	write-error "ERROR: output directory does not exist:  $OUTDIR"
	return
}

$OUTFILE="$OUTDIR\$CN.tar"

if(test-path $OUTFILE){
	return
}

$CNF_TEMPLATE=@'
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name

[req_distinguished_name]

[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = kubernetes
DNS.2 = kubernetes.default
'@
write-host "Generating SSL artifacts in $OUTDIR"


$CONFIGFILE="$OUTDIR\$CERTBASE-req.cnf"
$CAFILE="$OUTDIR\ca.pem"
$CAKEYFILE="$OUTDIR\ca-key.pem"
$KEYFILE="$OUTDIR\$CERTBASE-key.pem"
$CSRFILE="$OUTDIR\$CERTBASE.csr"
$PEMFILE="$OUTDIR\$CERTBASE.pem"

$CONTENTS="$CAFILE $KEYFILE $PEMFILE"


# Add SANs to openssl config
$CNF_TEMPLATE | out-file $CONFIGFILE  -encoding ASCII
$SANS -replace ",", "`n" | out-file $CONFIGFILE  -encoding ASCII -append

. $OPENSSL genrsa -out "$KEYFILE" 2048
. $OPENSSL req -new -key "$KEYFILE" -out "$CSRFILE" -subj "/CN=$CN" -config "$CONFIGFILE"
. $OPENSSL x509 -req -in "$CSRFILE" -CA "$CAFILE" -CAkey "$CAKEYFILE" -CAcreateserial -out "$PEMFILE" -days 365 -extensions v3_req -extfile "$CONFIGFILE"

. $ZIP a -ttar $OUTFILE .\$CAFILE
. $ZIP a -ttar $OUTFILE .\$KEYFILE
. $ZIP a -ttar $OUTFILE .\$PEMFILE

write-host "Bundled SSL artifacts into $OUTFILE"
write-host "$CONTENTS"