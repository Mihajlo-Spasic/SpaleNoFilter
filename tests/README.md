run the tests:
INTERNAL_SECRET=internal-service-to-service-secret-key go test ./... -timeout 120s


unit tests only:
go test ./... -run "^Test[^I]"


integration tests only:
INTERNAL_SECRET=internal-service-to-service-secret-key go test ./... -run "^TestIntegration" -timeout 120s
