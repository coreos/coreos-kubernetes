require 'openssl'

def create_ca_cert(outdir)
  paths = {
    :config => File.join(outdir, "ca.cnf"),
    :key => File.join(outdir, "ca-key.pem"),
    :pem => File.join(outdir, "ca.pem")
  }

  return paths if File.exist?(paths[:pem])

  $stdout << "Generating CA artifacts in #{outdir}\n"

  key = OpenSSL::PKey::RSA.new(2048)

  pem = OpenSSL::X509::Certificate.new
  pem.version = 3
  pem.serial = 1
  pem.subject = OpenSSL::X509::Name.parse('/CN=kub-ca')
  pem.issuer = pem.subject
  pem.not_before = Time.now
  pem.not_after  = Time.now + (10 * 365 * 24 * 60 * 60)
  pem.public_key = key.public_key

  ef = OpenSSL::X509::ExtensionFactory.new
  ef.subject_certificate = pem
  ef.issuer_certificate = pem

  pem.add_extension(ef.create_extension('basicConstraints', 'CA:TRUE', true))
  pem.add_extension(ef.create_extension('keyUsage', 'keyCertSign, cRLSign', true))
  pem.add_extension(ef.create_extension('subjectKeyIdentifier', 'hash', false))
  pem.add_extension(ef.create_extension('authorityKeyIdentifier', 'keyid:always', false))

  pem.sign(key, OpenSSL::Digest::SHA1.new)

  File.write(paths[:key], key.export)
  File.write(paths[:pem], pem.to_pem)

  return paths
end

def create_ssl_cert(
  outdir,
  basename,
  cn,
  ip_addrs=[],
  ca_cert_paths
)
  paths = {
    :key => File.join(outdir, "#{basename}-key.pem"),
    :pem => File.join(outdir, "#{basename}.pem"),
    :ca => ca_cert_paths[:pem],
    :ca_key => ca_cert_paths[:key]
  }

  return paths if File.exist?(paths[:pem])

  $stdout << "Generating SSL artifacts for #{basename} in #{outdir}\n"

  FileUtils.mkdir_p(outdir)

  key = OpenSSL::PKey::RSA.new(2048)

  ca = OpenSSL::X509::Certificate.new(File.read(ca_cert_paths[:pem]))
  ca_key = OpenSSL::PKey::RSA.new(File.read(ca_cert_paths[:key]))

  pem = OpenSSL::X509::Certificate.new
  pem.version = 3
  pem.serial = 2
  pem.subject = OpenSSL::X509::Name.parse("/CN=#{cn}")
  pem.issuer = ca.subject
  pem.public_key = key.public_key
  pem.not_before = Time.now
  pem.not_after = Time.now + (365 * 24 * 60 * 60)

  ef = OpenSSL::X509::ExtensionFactory.new
  ef.subject_certificate = pem
  ef.issuer_certificate = ca

  pem.add_extension(ef.create_extension('basicConstraints', 'CA:FALSE', true))
  pem.add_extension(ef.create_extension('keyUsage', 'digitalSignature, nonRepudiation, keyEncipherment', true))
  pem.add_extension(ef.create_extension('subjectKeyIdentifier', 'hash', false))

  alt_names = [
    ['DNS.1', 'kubernetes'],
    ['DNS.2', 'kubernetes.default'],
    ['DNS.3', 'kubernetes.default.svc'],
    ['DNS.4', 'kubernetes.default.svc.cluster.local']
  ].concat(
    ip_addrs.reduce([[], 1]) {|(rs, i), ip|
      [rs.push(["IP.#{i}", ip]), i+1]
    }.first
  )

  pem.add_extension(
    ef.create_extension(
      'subjectAltName',
      alt_names.map {|(k,v)| "#{k}:#{v}"}.join(','),
      true
    )
  )

  pem.sign(ca_key, OpenSSL::Digest::SHA256.new)

  File.write(paths[:key], key.export)
  File.write(paths[:pem], pem.to_pem)

  return paths
end
