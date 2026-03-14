# SpaleNoFilter testovi

## Run the tests

Pokreni sve testove:

```bash
INTERNAL_SECRET=internal-service-to-service-secret-key go test ./... -timeout 120s
```
Pokreni samo unit testove:

```bash
go test ./... -run "^Test[^I]"
```

Pokreni samo integration testove:

```bash
INTERNAL_SECRET=internal-service-to-service-secret-key go test ./... -run "^TestIntegration" -timeout 120s
```
# test
