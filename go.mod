module procoder

go 1.23

// A bare `go 1.23` is a floor, not a version: setup-go reads go.mod and
// installs whatever the newest 1.23.x is on the day the job runs, so the
// same commit builds with different compilers weeks apart. Patch releases
// change codegen and vet, and Go stamps the toolchain into the binary, so
// a rebuilt dist/ binary stops matching the committed one for no reason
// the source can explain. Bumping this is a commit, not a Tuesday.
toolchain go1.23.12
