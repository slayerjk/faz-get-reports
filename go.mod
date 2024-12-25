module github.com/slayerjk/faz-get-reports

go 1.22.3

require github.com/go-ldap/ldap v3.0.3+incompatible // direct

require (
	github.com/ncruces/go-sqlite3 v0.20.0
	github.com/slayerjk/go-logging v0.0.0-20241224092502-96a6d16224bc
	github.com/slayerjk/go-mailing v1.0.0
	github.com/slayerjk/go-vafswork v0.0.0-20241224093828-a8a16ff47237
	github.com/slayerjk/go-valdapwork v0.0.0-20241225061435-f948f8da2743
	github.com/slayerjk/go-vawebwork v0.0.0-20241224094416-d18d8d718125
)

require (
	github.com/ncruces/julianday v1.0.0 // indirect
	github.com/tetratelabs/wazero v1.8.1 // indirect
	golang.org/x/sys v0.26.0 // indirect
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
)
