# statespec

Stateful generative testing for Go

## Overview

This package provides a way to test a system via a set of properties (called `Commands`) that 
each interact with the system under test and optionally track state that has modified in the system.

Commands are composed into a `Spec` and `Spec.Run` will run the spec a number of times, randomly
choosing commands in the spec to run.

## Example

### Real World API

[Real World](https://github.com/gothinkster/realworld) is a project that defines an OpenAPI specification 
for a fictional web service. Various people have written UI clients against this spec and backend 
implementations of the spec.

We can use statespec to test a Real World backend.

```bash
# Start a separate terminal
#
# Clone a Go implementation - not affiliated with this project
git clone https://github.com/xesina/golang-echo-realworld-example-app.git
# Start the server - this runs on localhost:8585 and will write data to a file using sqlite3
go run main.go
```

In another terminal run the test

```bash
cd statespec
go run examples/realworldapi/realworldapi.go -n 10
```

