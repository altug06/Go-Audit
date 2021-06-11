module audit-client

require (
	github.com/elastic/go-libaudit v1.0.0
	github.com/google/uuid v1.1.1
	github.com/lunixbochs/struc v0.0.0-20200521075829-a4cb8d33dbbe // indirect
	github.com/pkg/errors v0.9.1 // indirect
)

go 1.15

replace github.com/elastic/go-libaudit => /root/elastic/go-libaudit
